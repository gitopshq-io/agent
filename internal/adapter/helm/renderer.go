package helm

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/gitopshq-io/agent/internal/adapter/render"
	"github.com/gitopshq-io/agent/internal/adapter/source"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func RenderFromArchive(chartTgz []byte, releaseName, namespace string, values map[string]any) ([]render.Manifest, error) {
	ch, err := loader.LoadArchive(bytes.NewReader(chartTgz))
	if err != nil {
		return nil, fmt.Errorf("load chart archive: %w", err)
	}
	return renderChart(ch, releaseName, namespace, values)
}

func RenderFromFiles(chartFiles []source.ChartFile, releaseName, namespace string, values map[string]any) ([]render.Manifest, error) {
	buffered := make([]*loader.BufferedFile, 0, len(chartFiles))
	for _, file := range chartFiles {
		buffered = append(buffered, &loader.BufferedFile{
			Name: file.Path,
			Data: []byte(file.Content),
		})
	}
	ch, err := loader.LoadFiles(buffered)
	if err != nil {
		return nil, fmt.Errorf("load chart files: %w", err)
	}
	return renderChart(ch, releaseName, namespace, values)
}

func renderChart(ch *chart.Chart, releaseName, namespace string, values map[string]any) ([]render.Manifest, error) {
	cfg := new(action.Configuration)
	install := action.NewInstall(cfg)
	install.ClientOnly = true
	install.DryRun = true
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.Replace = true

	rel, err := install.Run(ch, values)
	if err != nil {
		return nil, fmt.Errorf("helm template: %w", err)
	}
	return parseCombinedManifests(rel.Manifest), nil
}

func parseCombinedManifests(combined string) []render.Manifest {
	var manifests []render.Manifest
	docs := strings.Split(combined, "---")
	index := 0
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		path := fmt.Sprintf("manifest-%d", index)
		content := doc
		if strings.HasPrefix(doc, "# Source: ") {
			lines := strings.SplitN(doc, "\n", 2)
			path = strings.TrimSpace(strings.TrimPrefix(lines[0], "# Source: "))
			if len(lines) > 1 {
				content = strings.TrimSpace(lines[1])
			}
		}
		kind, name := extractKindName(content)
		manifests = append(manifests, render.Manifest{
			Path:    path,
			Kind:    kind,
			Name:    name,
			Content: content,
		})
		index++
	}
	return manifests
}

func extractKindName(content string) (string, string) {
	var meta struct {
		Kind     string `yaml:"kind"`
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
	}
	if err := yaml.Unmarshal([]byte(content), &meta); err != nil {
		return "Unknown", ""
	}
	kind := meta.Kind
	if kind == "" {
		kind = "Unknown"
	}
	return kind, meta.Metadata.Name
}
