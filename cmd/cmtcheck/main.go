// 주석 규칙
// - 파일 맨 위(package 앞)의 주석은 줄마다 80자 이내
// - 함수 바로 위 주석은 한 묶음 80자 이내, 동작 방식만 적는다
// - 그 밖의 주석(함수 안, 타입·상수·필드)은 위반
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

const maxRunes = 80

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

// 함수 doc 묶음을 모아 두고, 파일 머리·함수 doc이 아닌 묶음을 센다.
func checkFile(path string) int {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, path, nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	docs := map[*ast.CommentGroup]bool{}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Doc != nil {
			docs[fn.Doc] = true
		}
	}
	bad := 0
	report := func(g *ast.CommentGroup, why, text string) {
		fmt.Printf("%s:%d: %s: %s\n", path, fs.Position(g.Pos()).Line, why, text)
		bad++
	}
	for _, g := range f.Comments {
		if strings.HasPrefix(g.List[0].Text, "//go:") {
			continue
		}
		switch {
		case g.End() < f.Package:
			for _, c := range g.List {
				if t := commentText(c.Text); utf8.RuneCountInString(t) > maxRunes {
					report(g, fmt.Sprintf("머리 주석 %d자", utf8.RuneCountInString(t)), t)
				}
			}
		case docs[g]:
			t := ""
			for _, c := range g.List {
				t += commentText(c.Text)
			}
			if n := utf8.RuneCountInString(t); n > maxRunes {
				report(g, fmt.Sprintf("함수 주석 %d자", n), t)
			}
		default:
			report(g, "함수 설명 외 주석은 파일 맨 위로", commentText(g.List[0].Text))
		}
	}
	return bad
}

func commentText(c string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(c, "//"), "/*"), "*/"))
}
