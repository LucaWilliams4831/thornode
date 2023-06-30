package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"hash/fnv"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// -------------------------------------------------------------------------------------
// Flags
// -------------------------------------------------------------------------------------

var flagVersion *int

func init() {
	flagVersion = flag.Int("version", 0, "current version allowing changes")
}

// -------------------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------------------

var reVersionedName = regexp.MustCompile(`.*V([0-9]+)$`)

func isVersionedFunction(node ast.Node, fset *token.FileSet) (bool, int) {
	n, ok := node.(*ast.FuncDecl)
	var version string

	switch {
	case !ok:
		return false, 0
	case !reVersionedName.MatchString(n.Name.Name):
		// search receiever for a versioned struct name
		if n.Recv != nil {
			for _, r := range n.Recv.List {
				buf := new(bytes.Buffer)
				printer.Fprint(buf, fset, r.Type)
				if reVersionedName.MatchString(buf.String()) {
					// extract the version from the struct type
					version = reVersionedName.FindStringSubmatch(buf.String())[1]
					break
				}
			}
		}

		// if version was not found in receivers it is not versioned
		if version == "" {
			return false, 0
		}

	default:
		// extract the version from the function name
		version = reVersionedName.FindStringSubmatch(n.Name.Name)[1]
	}

	fnVersion, _ := strconv.Atoi(version)
	return true, fnVersion
}

func hasBuildFlags(file *ast.File) bool {
	for _, comment := range file.Comments {
		if strings.Contains(comment.Text(), "+build") {
			return true
		}
	}
	return false
}

// -------------------------------------------------------------------------------------
// Main
// -------------------------------------------------------------------------------------

func main() {
	// parse flags
	flag.Parse()

	fset := token.NewFileSet()
	pkgs := []*ast.Package{}

	// parse all subdirectories with go files
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			subPkgs, err := parser.ParseDir(fset, path, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			for _, pkg := range subPkgs {
				pkgs = append(pkgs, pkg)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error parsing files:", err)
		os.Exit(1)
	}

	// walk the ast and record all versioned functions
	fnsMap := map[token.Pos]ast.Node{}
	fnsDedupe := map[uint64][]string{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			// skip files with build flags (not mainnet)
			if hasBuildFlags(file) {
				// explicitly disallow build flags on handler files
				if strings.HasPrefix(file.Name.Name, "handler") {
					fmt.Println("Error: build flags are not allowed on handler files")
					os.Exit(1)
				}

				continue
			}

			// skip test files
			if strings.HasSuffix(fset.File(file.Pos()).Name(), "_test.go") {
				continue
			}

			ast.Inspect(file, func(node ast.Node) bool {
				// remove all comments
				v := reflect.ValueOf(node)
				if v.Kind() == reflect.Ptr && !v.IsNil() {
					v = v.Elem()
				}
				if v.IsValid() {
					for _, field := range []string{"Doc", "Comments"} {
						field := v.FieldByName(field)
						if field.IsValid() {
							field.Set(reflect.Zero(field.Type()))
						}
					}
				}

				isVersioned, version := isVersionedFunction(node, fset)
				if isVersioned {
					fn, ok := node.(*ast.FuncDecl)
					if !ok {
						panic("unreachable")
					}
					fnHash := fnv.New64a()
					name := []string{}
					if fn.Recv != nil {
						for _, r := range fn.Recv.List {
							buf := new(bytes.Buffer)
							printer.Fprint(buf, fset, r.Type)
							name = append(name, buf.String())
							printer.Fprint(fnHash, fset, r.Type)
						}
					}
					printer.Fprint(fnHash, fset, fn.Body)

					// dedupe by function receiver and body
					name = append(name, fn.Name.Name)
					fnsDedupe[fnHash.Sum64()] = append(fnsDedupe[fnHash.Sum64()], strings.Join(name, "."))

					// record all versioned functions outside current version
					if version != *flagVersion {
						fnsMap[node.Pos()] = node
					}
				}
				return true
			})
		}
	}

	// explicitly disallow duplicate versioned functions
	for _, fns := range fnsDedupe {
		if len(fns) > 1 {
			fmt.Fprintf(os.Stderr, "Error: duplicate versioned functions: %s\n", strings.Join(fns, ", "))

			// TODO: abort here after duplicate functions are removed from develop
			// os.Exit(1)
		}
	}

	// convert to slice for sorting
	fns := []ast.Node{}
	for _, fn := range fnsMap {
		fns = append(fns, fn)
	}

	// sort by function name in case filestructure changes
	sort.Slice(fns, func(i, j int) bool {
		fi, ok := fns[i].(*ast.FuncDecl)
		if !ok {
			panic("unreachable")
		}
		fj, ok := fns[j].(*ast.FuncDecl)
		if !ok {
			panic("unreachable")
		}

		ii := new(bytes.Buffer)
		if fi.Recv != nil {
			for _, f := range fi.Recv.List {
				printer.Fprint(ii, fset, f.Type)
			}
		}
		ii.WriteString(fi.Name.Name)
		printer.Fprint(ii, fset, fi.Type)

		jj := new(bytes.Buffer)
		if fj.Recv != nil {
			for _, f := range fj.Recv.List {
				printer.Fprint(jj, fset, f.Type)
			}
		}
		jj.WriteString(fj.Name.Name)
		printer.Fprint(jj, fset, fj.Type)

		return strings.ToLower(ii.String()) < strings.ToLower(jj.String())
	})

	// print package so gofumpt can format
	fmt.Println("package main")

	// print the versioned functions
	for _, fn := range fns {
		pos := fset.Position(fn.Pos())
		fmt.Printf("// %s:%d\n", pos.Filename, pos.Line)
		printer.Fprint(os.Stdout, fset, fn)
		fmt.Printf("\n\n")
	}
}
