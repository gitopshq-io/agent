package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gitopshq-io/agent/internal/domain"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/registry"
)

type SecretReader interface {
	ReadSecretData(ctx context.Context, ref domain.CredentialRef) (map[string][]byte, error)
}

type Loader struct {
	WorkDir string
	Secrets SecretReader
}

func (l Loader) CheckoutGit(ctx context.Context, source domain.SourceRef) (string, func(), error) {
	if source.URL == "" {
		return "", nil, fmt.Errorf("source url is required")
	}
	if source.ResolvedRevision == "" {
		return "", nil, fmt.Errorf("resolved revision is required for git sources")
	}

	root, err := os.MkdirTemp(defaultWorkDir(l.WorkDir), "gitopshq-source-*")
	if err != nil {
		return "", nil, fmt.Errorf("create source workspace: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(root) }

	cloneOptions := &git.CloneOptions{
		URL: source.URL,
	}
	if source.CredentialRef != nil {
		auth, authErr := l.gitAuth(ctx, root, source.URL, *source.CredentialRef)
		if authErr != nil {
			cleanup()
			return "", nil, authErr
		}
		cloneOptions.Auth = auth
	}

	repository, err := git.PlainCloneContext(ctx, root, false, cloneOptions)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("clone source repository: %w", err)
	}
	revision, err := repository.ResolveRevision(plumbing.Revision(source.ResolvedRevision))
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("resolve revision %q: %w", source.ResolvedRevision, err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("get worktree: %w", err)
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Hash: *revision, Force: true}); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("checkout revision %q: %w", source.ResolvedRevision, err)
	}
	return root, cleanup, nil
}

func (l Loader) PullHelmOCI(ctx context.Context, source domain.SourceRef) ([]byte, error) {
	ref, err := helmOCIRef(source)
	if err != nil {
		return nil, err
	}
	options := []registry.ClientOption{
		registry.ClientOptWriter(io.Discard),
	}
	if source.CredentialRef != nil {
		secretData, err := l.Secrets.ReadSecretData(ctx, *source.CredentialRef)
		if err != nil {
			return nil, err
		}
		username, password := secretUsernamePassword(secretData)
		if password != "" {
			options = append(options, registry.ClientOptBasicAuth(username, password))
		}
	}
	if strings.Contains(ref, "localhost") || strings.Contains(ref, "127.0.0.1") {
		options = append(options, registry.ClientOptPlainHTTP())
	}
	client, err := registry.NewClient(options...)
	if err != nil {
		return nil, fmt.Errorf("create helm registry client: %w", err)
	}
	result, err := client.Pull(ref)
	if err != nil {
		return nil, fmt.Errorf("pull helm oci chart: %w", err)
	}
	return result.Chart.Data, nil
}

func (l Loader) ResolveValues(ctx context.Context, ref *domain.ValuesRef) (map[string]any, error) {
	if ref == nil {
		return nil, nil
	}
	out := cloneMap(ref.InlineValues)
	if ref.CredentialRef == nil {
		if err := verifyInlineValuesDigest(out, ref.Digest); err != nil {
			return nil, err
		}
		return out, nil
	}
	if ref.Digest == "" {
		return nil, errors.New("values digest is required for credential-backed values")
	}
	secretData, err := l.Secrets.ReadSecretData(ctx, *ref.CredentialRef)
	if err != nil {
		return nil, err
	}
	payload, err := resolveSecretPayload(secretData, ref.CredentialRef.Key, []string{"values.yaml", "values", "values.yml", "values.json"})
	if err != nil {
		return nil, err
	}
	if payload == "" {
		return nil, errors.New("values payload is empty")
	}
	if err := verifyDigest([]byte(payload), ref.Digest); err != nil {
		return nil, err
	}
	decoded := map[string]any{}
	if err := yaml.Unmarshal([]byte(payload), &decoded); err != nil {
		return nil, fmt.Errorf("parse values payload: %w", err)
	}
	return mergeMaps(decoded, out), nil
}

func LoadFiles(root string) (map[string]string, error) {
	resolvedRoot, err := ResolvePath(root, "")
	if err != nil {
		return nil, err
	}
	files := make(map[string]string)
	err = filepath.WalkDir(resolvedRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			rel, relErr := filepath.Rel(resolvedRoot, path)
			if relErr != nil {
				rel = path
			}
			return fmt.Errorf("symlinked source path %q is not allowed", filepath.ToSlash(rel))
		}
		rel, err := filepath.Rel(resolvedRoot, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(content)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func LoadChartFiles(root string) ([]ChartFile, error) {
	files, err := LoadFiles(root)
	if err != nil {
		return nil, err
	}
	out := make([]ChartFile, 0, len(files))
	for path, content := range files {
		out = append(out, ChartFile{Path: path, Content: content})
	}
	return out, nil
}

type ChartFile struct {
	Path    string
	Content string
}

func defaultWorkDir(configured string) string {
	if configured != "" {
		return configured
	}
	return os.TempDir()
}

func ResolvePath(root, sourcePath string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("source root is required")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve source root %q: %w", root, err)
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return "", fmt.Errorf("resolve absolute source root %q: %w", root, err)
	}
	target := resolvedRoot
	if sourcePath != "" {
		target = filepath.Join(resolvedRoot, sourcePath)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("resolve source path %q: %w", sourcePath, err)
	}
	resolvedTarget, err = filepath.Abs(resolvedTarget)
	if err != nil {
		return "", fmt.Errorf("resolve absolute source path %q: %w", sourcePath, err)
	}
	if !isWithinRoot(resolvedRoot, resolvedTarget) {
		return "", fmt.Errorf("source path %q escapes repository root", sourcePath)
	}
	return resolvedTarget, nil
}

func helmOCIRef(source domain.SourceRef) (string, error) {
	base := strings.TrimSpace(source.URL)
	if base == "" {
		return "", fmt.Errorf("source url is required")
	}
	if source.Chart != "" && !strings.HasSuffix(base, "/"+source.Chart) {
		base = strings.TrimRight(base, "/") + "/" + source.Chart
	}
	if !strings.HasPrefix(base, "oci://") {
		base = "oci://" + strings.TrimPrefix(base, "oci://")
	}
	switch {
	case source.ResolvedDigest != "":
		return base + "@" + source.ResolvedDigest, nil
	case source.ResolvedRevision != "":
		return base + ":" + source.ResolvedRevision, nil
	default:
		return "", fmt.Errorf("resolved revision or digest is required for helm oci sources")
	}
}

func (l Loader) gitAuth(ctx context.Context, workspace, rawURL string, ref domain.CredentialRef) (transport.AuthMethod, error) {
	secretData, err := l.Secrets.ReadSecretData(ctx, ref)
	if err != nil {
		return nil, err
	}
	if privateKey := firstSecretValue(secretData, "ssh-privatekey", "sshPrivateKey"); privateKey != "" {
		auth, err := gitssh.NewPublicKeys(defaultString(firstSecretValue(secretData, "username"), "git"), []byte(privateKey), firstSecretValue(secretData, "passphrase"))
		if err != nil {
			return nil, fmt.Errorf("create ssh auth from secret %s/%s: %w", ref.Namespace, ref.SecretName, err)
		}
		knownHosts := firstSecretValue(secretData, "known_hosts", "knownHosts")
		if knownHosts != "" {
			knownHostsPath := filepath.Join(workspace, "known_hosts")
			if err := os.WriteFile(knownHostsPath, []byte(knownHosts), 0o600); err != nil {
				return nil, fmt.Errorf("write known_hosts: %w", err)
			}
			callback, err := gitssh.NewKnownHostsCallback(knownHostsPath)
			if err != nil {
				return nil, fmt.Errorf("create known_hosts callback: %w", err)
			}
			auth.HostKeyCallback = callback
		}
		return auth, nil
	}
	username, password := secretUsernamePassword(secretData)
	if password == "" {
		return nil, nil
	}
	if parsed, err := url.Parse(rawURL); err == nil && parsed.User != nil && username == "" {
		username = parsed.User.Username()
	}
	return &githttp.BasicAuth{
		Username: defaultString(username, "x-token"),
		Password: password,
	}, nil
}

func resolveSecretPayload(secretData map[string][]byte, explicitKey string, fallbacks []string) (string, error) {
	if explicitKey != "" {
		payload, ok := secretData[explicitKey]
		if !ok {
			return "", fmt.Errorf("secret key %q not found", explicitKey)
		}
		return string(payload), nil
	}
	for _, key := range fallbacks {
		if payload, ok := secretData[key]; ok {
			return string(payload), nil
		}
	}
	return "", nil
}

func secretUsernamePassword(secretData map[string][]byte) (string, string) {
	username := firstSecretValue(secretData, "username")
	password := firstSecretValue(secretData, "password")
	if password == "" {
		password = firstSecretValue(secretData, "token")
	}
	return defaultString(username, "x-token"), password
}

func firstSecretValue(secretData map[string][]byte, keys ...string) string {
	for _, key := range keys {
		if value, ok := secretData[key]; ok && len(value) > 0 {
			return strings.TrimSpace(string(value))
		}
	}
	return ""
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func mergeMaps(base, overrides map[string]any) map[string]any {
	if len(base) == 0 {
		return cloneMap(overrides)
	}
	out := cloneMap(base)
	for key, value := range overrides {
		if nestedBase, ok := out[key].(map[string]any); ok {
			if nestedOverride, ok := value.(map[string]any); ok {
				out[key] = mergeMaps(nestedBase, nestedOverride)
				continue
			}
		}
		out[key] = value
	}
	return out
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func verifyInlineValuesDigest(values map[string]any, expected string) error {
	if expected == "" {
		return nil
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("marshal inline values: %w", err)
	}
	return verifyDigest(payload, expected)
}

func verifyDigest(payload []byte, expected string) error {
	if expected == "" {
		return nil
	}
	sum := sha256.Sum256(payload)
	computed := hex.EncodeToString(sum[:])
	normalized := strings.ToLower(strings.TrimSpace(expected))
	if normalized == computed || normalized == "sha256:"+computed {
		return nil
	}
	return fmt.Errorf("digest mismatch")
}

func isWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
