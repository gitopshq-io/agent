package kustomize

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gitopshq-io/agent/internal/adapter/render"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
)

func Build(files map[string]string) ([]render.Manifest, error) {
	fs := filesys.MakeFsInMemory()
	for path, content := range files {
		dir := dirOf(path)
		if dir != "." {
			_ = fs.MkdirAll(dir)
		}
		if err := fs.WriteFile(path, []byte(content)); err != nil {
			return nil, fmt.Errorf("write in-memory file %s: %w", path, err)
		}
	}

	root := findKustomizationRoot(files)
	kustomizer := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	resMap, err := kustomizer.Run(fs, root)
	if err != nil {
		return nil, fmt.Errorf("kustomize build: %w", err)
	}

	manifests := make([]render.Manifest, 0, len(resMap.Resources()))
	for _, resource := range resMap.Resources() {
		content, err := resource.AsYAML()
		if err != nil {
			return nil, fmt.Errorf("render kustomize resource: %w", err)
		}
		kind, name := extractKindName(string(content))
		manifests = append(manifests, render.Manifest{
			Path:    fmt.Sprintf("%s/%s", strings.ToLower(kind), name),
			Kind:    kind,
			Name:    name,
			Content: strings.TrimSpace(string(content)),
		})
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].Path < manifests[j].Path })
	return manifests, nil
}

func findKustomizationRoot(files map[string]string) string {
	for path := range files {
		base := pathBase(path)
		if base == "kustomization.yaml" || base == "kustomization.yml" || base == "Kustomization" {
			return dirOf(path)
		}
	}
	return "."
}

func pathBase(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[index+1:]
	}
	return path
}

func dirOf(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[:index]
	}
	return "."
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
