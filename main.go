package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
)

func main() {
	inputPath := flag.String("input", "", "Input entry Go file path")
	outputDir := flag.String("output", "output", "Output directory for filtered source files")
	flag.Parse()

	if *inputPath == "" {
		log.Println("Please specify the input Go file path using -input flag")
		return
	}

	usedSymbols, decls, err := CollectUsedDeclarations(*inputPath)
	if err != nil {
		log.Println("Analysis failed:", err)
		return
	}

	log.Println("Recursive dependency declarations in the entry file:")
	for name := range usedSymbols {
		log.Println("  ", name)
	}

	outPath := filepath.Join(*outputDir, filepath.Base(*inputPath))
	if err := WriteFilteredSource(*inputPath, outPath, decls); err != nil {
		log.Println("Write failed:", err)
		return
	}
	autoFixImports(outPath)
	log.Println("Cut successfully, ", outPath)
}

func CollectUsedDeclarations(entryFile string) (map[string]bool, []ast.Decl, error) {
	fset := token.NewFileSet()

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps,
		Fset: fset,
		Dir:  filepath.Dir(entryFile),
		Env:  os.Environ(),
	}

	pkgs, err := packages.Load(cfg, "file="+entryFile)
	if err != nil || len(pkgs) == 0 {
		return nil, nil, fmt.Errorf("failed to load package: %w", err)
	}
	pkg := pkgs[0]
	info := pkg.TypesInfo
	files := pkg.Syntax

	var entryAST *ast.File
	for _, f := range files {
		if fset.Position(f.Pos()).Filename == entryFile {
			entryAST = f
			break
		}
	}
	if entryAST == nil {
		return nil, nil, fmt.Errorf("unable to find the entrance AST")
	}

	visited := map[types.Object]bool{}
	used := map[string]bool{}
	declMap := map[string]ast.Decl{}

	var visit func(obj types.Object)
	visit = func(obj types.Object) {
		if obj == nil || visited[obj] {
			return
		}
		visited[obj] = true
		used[obj.Name()] = true

		for _, file := range files {
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if d.Name.Name == obj.Name() {
						declMap[obj.Name()] = d
						if d.Body != nil {
							ast.Inspect(d.Body, func(n ast.Node) bool {
								if id, ok := n.(*ast.Ident); ok {
									ref := info.Uses[id]
									if ref == nil {
										ref = info.Defs[id]
									}
									visit(ref)
								}
								return true
							})
						}
					}
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							if s.Name.Name == obj.Name() {
								declMap[obj.Name()] = d
								if structType, ok := s.Type.(*ast.StructType); ok {
									for _, field := range structType.Fields.List {
										visitTypeExpr(field.Type, info, visit)
									}
								}
							}
						case *ast.ValueSpec:
							for _, name := range s.Names {
								if name.Name == obj.Name() {
									declMap[obj.Name()] = d
									if s.Type != nil {
										visitTypeExpr(s.Type, info, visit)
									}
									for _, val := range s.Values {
										visitExpr(val, info, visit)
									}
								}
							}
						}
					}
				}
			}
		}
	}

	for _, decl := range entryAST.Decls {
		ast.Inspect(decl, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				obj := info.Uses[id]
				if obj == nil {
					obj = info.Defs[id]
				}
				visit(obj)
			}
			return true
		})
	}

	var decls []ast.Decl
	for _, d := range declMap {
		decls = append(decls, d)
	}

	return used, decls, nil
}

func visitTypeExpr(expr ast.Expr, info *types.Info, visit func(types.Object)) {
	switch e := expr.(type) {
	case *ast.Ident:
		visit(info.Uses[e])
	case *ast.StarExpr:
		visitTypeExpr(e.X, info, visit)
	case *ast.ArrayType:
		visitTypeExpr(e.Elt, info, visit)
	case *ast.MapType:
		visitTypeExpr(e.Key, info, visit)
		visitTypeExpr(e.Value, info, visit)
	case *ast.SelectorExpr:

		visitTypeExpr(e.X, info, visit)
	case *ast.FuncType:
		for _, field := range e.Params.List {
			visitTypeExpr(field.Type, info, visit)
		}
		for _, field := range e.Results.List {
			visitTypeExpr(field.Type, info, visit)
		}
	case *ast.InterfaceType:

	case *ast.ChanType:
		visitTypeExpr(e.Value, info, visit)
	case *ast.Ellipsis:
		visitTypeExpr(e.Elt, info, visit)
	}
}

func visitExpr(expr ast.Expr, info *types.Info, visit func(types.Object)) {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		visitTypeExpr(e.Type, info, visit)
		for _, elt := range e.Elts {
			visitExpr(elt, info, visit)
		}
	case *ast.CallExpr:
		visitExpr(e.Fun, info, visit)
		for _, arg := range e.Args {
			visitExpr(arg, info, visit)
		}
	case *ast.UnaryExpr:
		visitExpr(e.X, info, visit)
	case *ast.BinaryExpr:
		visitExpr(e.X, info, visit)
		visitExpr(e.Y, info, visit)
	case *ast.ParenExpr:
		visitExpr(e.X, info, visit)
	case *ast.KeyValueExpr:
		visitExpr(e.Key, info, visit)
		visitExpr(e.Value, info, visit)
	case *ast.Ident:
		visit(info.Uses[e])
	case *ast.SelectorExpr:
		visitExpr(e.X, info, visit)
	case *ast.IndexExpr:
		visitExpr(e.X, info, visit)
		visitExpr(e.Index, info, visit)
	case *ast.SliceExpr:
		visitExpr(e.X, info, visit)
		if e.Low != nil {
			visitExpr(e.Low, info, visit)
		}
		if e.High != nil {
			visitExpr(e.High, info, visit)
		}
		if e.Max != nil {
			visitExpr(e.Max, info, visit)
		}
	case *ast.TypeAssertExpr:
		visitExpr(e.X, info, visit)
	case *ast.BasicLit, *ast.BadExpr, *ast.FuncLit, *ast.Ellipsis:
	default:
	}
}

func WriteFilteredSource(entryFile, outFile string, decls []ast.Decl) error {
	src, err := os.ReadFile(entryFile)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, entryFile, src, parser.AllErrors)
	if err != nil {
		return err
	}

	file.Decls = decls

	if err := os.MkdirAll(filepath.Dir(outFile), 0755); err != nil {
		return err
	}
	fout, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer fout.Close()
	return format.Node(fout, fset, file)
}

func autoFixImports(filePath string) error {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	opt := &imports.Options{
		Comments:   true,
		TabWidth:   8,
		TabIndent:  true,
		FormatOnly: false,
	}

	fixed, err := imports.Process(filePath, src, opt)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, fixed, 0644)
}
