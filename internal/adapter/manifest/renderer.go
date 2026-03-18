package manifest

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gitopshq-io/agent/internal/adapter/render"
	"gopkg.in/yaml.v3"
)

func RenderFiles(files map[string]string) []render.Manifest {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	manifests := make([]render.Manifest, 0, len(paths))
	for _, path := range paths {
		if !isYAMLFile(path) {
			continue
		}
		docs := strings.Split(files[path], "\n---")
		for index, doc := range docs {
			doc = strings.TrimSpace(doc)
			if doc == "" {
				continue
			}
			kind, name := extractKindName(doc)
			docPath := path
			if len(docs) > 1 {
				docPath = fmt.Sprintf("%s#%d", path, index)
			}
			manifests = append(manifests, render.Manifest{
				Path:    docPath,
				Kind:    kind,
				Name:    name,
				Content: doc,
			})
		}
	}
	return manifests
}

func isYAMLFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
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
