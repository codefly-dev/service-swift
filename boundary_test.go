package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenImportFragments identifies packages that would give this manifest
// producer a transport, repository, or reconciler responsibility it must not
// own: Git clients, GitHub, and Argo CD/Flux reconciliation APIs.
var forbiddenImportFragments = []string{
	"go-git/go-git",
	"go-github",
	"argoproj",
	"argo-cd",
	"fluxcd",
}

// TestRuntimeSourceHasNoTransportIntegration guards the boundary in the issue:
// the plugin renders manifests but never learns how they are transported,
// reviewed, or reconciled. It parses the plugin's own (non-test) sources and
// fails if any imports a repository or reconciler client.
func TestRuntimeSourceHasNoTransportIntegration(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			for _, fragment := range forbiddenImportFragments {
				if strings.Contains(path, fragment) {
					t.Errorf("%s imports transport/reconciler package %q", name, path)
				}
			}
		}
	}
}
