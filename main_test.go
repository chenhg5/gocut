package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectUsedDeclarations(t *testing.T) {
	entry := filepath.Join("test", "entry.go")
	absEntry, err := filepath.Abs(entry)
	if err != nil {
		t.Fatal("failed to get absolute path:", err)
	}

	used, decls, err := CollectUsedDeclarations(absEntry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for expected symbols
	expectedSymbols := []string{"MainFunc", "helper", "MyStruct"}
	for _, sym := range expectedSymbols {
		if !used[sym] {
			t.Errorf("expected symbol %s not found in used set", sym)
		}
	}

	// Check decls has at least all expected ones
	declNames := map[string]bool{}
	for _, decl := range decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			declNames[d.Name.Name] = true
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if vs, ok := spec.(*ast.TypeSpec); ok {
					declNames[vs.Name.Name] = true
				}
			}
		}
	}
	for _, sym := range expectedSymbols {
		if !declNames[sym] {
			t.Errorf("expected decl for %s not found", sym)
		}
	}
}

func TestWriteFilteredSource(t *testing.T) {
	entry := filepath.Join("test", "entry.go")
	absEntry, err := filepath.Abs(entry)
	if err != nil {
		t.Fatal("failed to get absolute path:", err)
	}
	out := filepath.Join(t.TempDir(), "out.go")

	_, decls, err := CollectUsedDeclarations(absEntry)
	if err != nil {
		t.Fatalf("CollectUsedDeclarations failed: %v", err)
	}

	if err := WriteFilteredSource(entry, out, decls); err != nil {
		t.Fatalf("WriteFilteredSource failed: %v", err)
	}

	// Confirm output file exists and non-empty
	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading output failed: %v", err)
	}
	if !strings.Contains(string(content), "MainFunc") {
		t.Errorf("output does not contain MainFunc")
	}
}

func TestAutoFixImports(t *testing.T) {
	entry := filepath.Join("test", "entry.go")
	absEntry, err := filepath.Abs(entry)
	if err != nil {
		t.Fatal("failed to get absolute path:", err)
	}
	out := filepath.Join(t.TempDir(), "out.go")

	_, decls, err := CollectUsedDeclarations(absEntry)
	if err != nil {
		t.Fatalf("CollectUsedDeclarations failed: %v", err)
	}

	if err := WriteFilteredSource(entry, out, decls); err != nil {
		t.Fatalf("WriteFilteredSource failed: %v", err)
	}

	if err := autoFixImports(out); err != nil {
		t.Fatalf("autoFixImports failed: %v", err)
	}

	// Confirm import is auto-fixed
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, out, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports failed: %v", err)
	}
	if len(node.Imports) == 0 {
		t.Errorf("expected imports to be fixed, but got none")
	}
}
