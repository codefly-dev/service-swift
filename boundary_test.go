package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenImportFragments identifies packages that would give this manifest
// producer a transport, repository, or reconciler responsibility it must not
// own: Git clients, GitHub, Argo CD/Flux reconciliation APIs, and direct
// cluster access via kubeconfig/client-go.
var forbiddenImportFragments = []string{
	"go-git/go-git",
	"go-github",
	"argoproj",
	"argo-cd",
	"fluxcd",
	"k8s.io/client-go",
	"k8s.io/kubectl",
	"clientcmd",
}

// scanForbiddenImports walks root and returns a violation for every non-test
// Go source that imports a transport, repository, or reconciler package. It
// descends into subpackages so the boundary holds for the whole module, not
// just its top-level files.
func scanForbiddenImports(root string) ([]string, error) {
	fset := token.NewFileSet()
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			imported := strings.Trim(spec.Path.Value, `"`)
			for _, fragment := range forbiddenImportFragments {
				if strings.Contains(imported, fragment) {
					violations = append(violations, fmt.Sprintf("%s imports transport/reconciler package %q", path, imported))
				}
			}
		}
		return nil
	})
	return violations, err
}

// TestRuntimeSourceHasNoTransportIntegration guards the boundary in the issue:
// the plugin renders manifests but never learns how they are transported,
// reviewed, or reconciled.
func TestRuntimeSourceHasNoTransportIntegration(t *testing.T) {
	violations, err := scanForbiddenImports(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Error(violation)
	}
}

// TestScanForbiddenImportsDetectsNestedAndKubeconfig ensures the guard catches
// a forbidden import that lives in a subpackage (not just top-level files) and
// that direct kubeconfig/cluster access counts as a violation.
func TestScanForbiddenImportsDetectsNestedAndKubeconfig(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "internal", "promotion")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	source := "package promotion\n\nimport _ \"k8s.io/client-go/tools/clientcmd\"\n"
	if err := os.WriteFile(filepath.Join(sub, "reconcile.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	violations, err := scanForbiddenImports(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) == 0 {
		t.Fatal("scanner missed a kubeconfig import in a subpackage")
	}
}

// TestScanForbiddenImportsAllowsCleanTree confirms the guard does not flag
// ordinary manifest-producer dependencies.
func TestScanForbiddenImportsAllowsCleanTree(t *testing.T) {
	root := t.TempDir()
	source := "package svc\n\nimport _ \"sigs.k8s.io/kustomize/api/krusty\"\n"
	if err := os.WriteFile(filepath.Join(root, "svc.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	violations, err := scanForbiddenImports(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("clean tree flagged: %v", violations)
	}
}
