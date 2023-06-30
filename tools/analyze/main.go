package main

import (
	"errors"
	"go/ast"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var reVersionedName = regexp.MustCompile(`.*V([0-9]+)$`)

// -------------------------------------------------------------------------------------
// MapIteration
// -------------------------------------------------------------------------------------

func MapIteration(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	// track lines with the expected ignore comment
	type ignorePos struct {
		file string
		line int
	}
	ignore := map[ignorePos]bool{}

	// one pass to find all comments
	inspect.Preorder([]ast.Node{(*ast.File)(nil)}, func(node ast.Node) {
		n, ok := node.(*ast.File)
		if !ok {
			panic("node was not *ast.File")
		}
		for _, c := range n.Comments {
			if strings.Contains(c.Text(), "analyze-ignore(map-iteration)") {
				p := pass.Fset.Position(c.Pos())
				ignore[ignorePos{p.Filename, p.Line + strings.Count(c.Text(), "\n")}] = true
			}
		}
	})

	inspect.Preorder([]ast.Node{(*ast.RangeStmt)(nil)}, func(node ast.Node) {
		n, ok := node.(*ast.RangeStmt)
		if !ok {
			panic("node was not *ast.RangeStmt")
		}
		// skip if this is not a range over a map
		if !strings.HasPrefix(pass.TypesInfo.TypeOf(n.X).String(), "map") {
			return
		}

		// skip if this is a test file
		p := pass.Fset.Position(n.Pos())
		if strings.HasSuffix(p.Filename, "_test.go") {
			return
		}

		// skip if the previous line contained the ignore comment
		if ignore[ignorePos{p.Filename, p.Line}] {
			return
		}

		pass.Reportf(node.Pos(), "found map iteration")
	})

	return nil, nil
}

// -------------------------------------------------------------------------------------
// VersionSwitch
// -------------------------------------------------------------------------------------

func VersionSwitch(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	// find all switch cases
	inspect.Preorder([]ast.Node{(*ast.CaseClause)(nil)}, func(node ast.Node) {
		n, ok := node.(*ast.CaseClause)
		if !ok {
			panic("node was not *ast.CaseClause")
		}

		for _, e := range n.List {
			version := ""
			cmpFn := ""
			var parent ast.Node

			ast.Inspect(e, func(n ast.Node) bool {
				if n == nil || version != "" {
					return false
				}
				if c, ok := n.(*ast.CallExpr); ok {

					// extract the version from semver.MustParse argument
					if s, ok := c.Fun.(*ast.SelectorExpr); ok {
						if x, ok := s.X.(*ast.Ident); ok {
							if x.Name == "semver" && s.Sel.Name == "MustParse" {
								if l, ok := c.Args[0].(*ast.BasicLit); ok {
									version = l.Value
									return false
								}
							}
						}
					}

					// extract the comparison function name
					fnType := pass.TypesInfo.TypeOf(c.Fun)
					if fnType.String() == "func(o github.com/blang/semver.Version) bool" {
						switch ft := c.Fun.(type) {
						case *ast.Ident:
							cmpFn = ft.Name
						case *ast.SelectorExpr:
							cmpFn = ft.Sel.Name
						}
						parent = n
					}
				}

				return true
			})
			if version == "" {
				continue
			}

			// ensure version switch is using GTE
			if cmpFn != "GTE" {
				pass.Reportf(parent.Pos(), "must use GTE in version switch")
			}

			// extract the minor version
			minor := strings.Split(version, ".")[1]

			// extract versioned functions called in the case body
			for _, s := range n.Body {
				ast.Inspect(s, func(n ast.Node) bool {
					if n == nil {
						return false
					}
					if c, ok := n.(*ast.CallExpr); ok {
						// extract function names from the body
						vFn := ""
						switch ft := c.Fun.(type) {
						case *ast.Ident:
							vFn = ft.Name
						case *ast.SelectorExpr:
							vFn = ft.Sel.Name
						default:
							return true
						}

						// verify function versions match
						v := reVersionedName.FindStringSubmatch(vFn)
						if len(v) == 2 && v[1] != minor {
							pass.Reportf(e.Pos(), "bad version switch body: %s != %s", v[1], minor)
						}
					}
					return true
				})
			}
		}
	})

	return nil, nil
}

// -------------------------------------------------------------------------------------
// Main
// -------------------------------------------------------------------------------------

func main() {
	multichecker.Main(
		&analysis.Analyzer{
			Name:     "map_iteration",
			Doc:      "fails on uncommented map iterations",
			Requires: []*analysis.Analyzer{inspect.Analyzer},
			Run:      MapIteration,
		},
		&analysis.Analyzer{
			Name:     "switch_version",
			Doc:      "fails on bad version switches",
			Requires: []*analysis.Analyzer{inspect.Analyzer},
			Run:      VersionSwitch,
		},
	)
}
