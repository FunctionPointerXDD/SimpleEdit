// cmtcheck는 주석 규칙(함수 안 금지, 50자 이내)을 검사한다.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// 주석 한 묶음의 최대 글자 수
const maxRunes = 50

// 검사하지 않는 최상위 폴더
var skipDirs = map[string]bool{"frontend": true, "tools": true, "build": true, "dist": true, "scripts": true}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	bad := 0
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() && (skipDirs[rel] || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".") && rel != ".") {
			return filepath.SkipDir
		}
		if strings.HasSuffix(p, ".go") {
			bad += checkFile(p)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println("위반", bad)
	if bad > 0 {
		os.Exit(1)
	}
}

// checkFile은 한 파일의 위반 수를 출력하고 돌려준다.
func checkFile(path string) int {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, path, nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	var bodies [][2]token.Pos
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Body != nil {
				bodies = append(bodies, [2]token.Pos{x.Body.Lbrace, x.Body.Rbrace})
			}
		case *ast.FuncLit:
			bodies = append(bodies, [2]token.Pos{x.Body.Lbrace, x.Body.Rbrace})
		}
		return true
	})
	bad := 0
	for _, g := range f.Comments {
		if strings.HasPrefix(g.List[0].Text, "//go:") {
			continue
		}
		text := ""
		for _, c := range g.List {
			text += strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), "/*"))
		}
		line := fs.Position(g.Pos()).Line
		for _, b := range bodies {
			if g.Pos() > b[0] && g.Pos() < b[1] {
				fmt.Printf("%s:%d: 함수 안 주석: %s\n", path, line, text)
				bad++
				break
			}
		}
		if n := utf8.RuneCountInString(text); n > maxRunes {
			fmt.Printf("%s:%d: %d자: %s\n", path, line, n, text)
			bad++
		}
	}
	return bad
}
