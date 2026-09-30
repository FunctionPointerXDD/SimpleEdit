// apispec: 루트 패키지(App 바인딩)의 Go 소스를 읽어 백엔드 API 명세를 만든다.
// 출력: docs/api.json(사람·도구용 명세), frontend/src/apiSpec.js(이벤트 이름·안내 문장·JSDoc 타입).
// 이벤트 내용 타입(ProgressEvent)은 Wails 바인딩(models.ts)에 없어서 apiSpec.js typedef로만 제공한다.
// 명세는 손으로 쓰지 않는다. Go 코드가 원본이고, -check는 두 파일이 코드와 다르면 실패한다.
// 메서드: *App의 공개 메서드(Wails가 바인딩하는 것). 타입: 메서드·이벤트에서 닿는 구조체만.
// 이벤트: progressEmitter(ev…)로 보내는 ev… 상수. 내용은 progressEmitter가 보내는 구조체다.
// 오류: msg… 상수. 백엔드 오류는 이 문장 그대로 프론트에 온다.
// 노트: 각 파일 맨 위 주석(좌표·단위·동작 규칙)을 그대로 옮긴다.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	module     = "simpleEdit/"
	emitter    = "progressEmitter"
	specPath   = "docs/api.json"
	jsPath     = "frontend/src/apiSpec.js"
	regenerate = "go run ./cmd/apispec"
)

type Field struct {
	JSON string `json:"json"`
	Go   string `json:"go"`
	Type string `json:"type"`
}

type Method struct {
	Name    string   `json:"name"`
	Doc     string   `json:"doc,omitempty"`
	Params  []string `json:"params"`
	Returns string   `json:"returns,omitempty"`
	Error   bool     `json:"error"`
}

type Event struct {
	Name    string `json:"name"`
	Const   string `json:"const"`
	Payload string `json:"payload"`
}

type Spec struct {
	Generated string              `json:"generated"`
	Notes     map[string][]string `json:"notes"`
	Methods   []Method            `json:"methods"`
	Types     map[string][]Field  `json:"types"`
	Events    []Event             `json:"events"`
	Errors    map[string]string   `json:"errors"`
}

type pkg struct {
	files   []*ast.File
	structs map[string]*ast.StructType
	imports map[string]string
}

type gen struct {
	fs    *token.FileSet
	root  string
	pkgs  map[string]*pkg
	types map[string][]Field
}

func main() {
	check := flag.Bool("check", false, "파일을 쓰지 않고 코드와 다르면 실패")
	flag.Parse()
	g := &gen{fs: token.NewFileSet(), root: ".", pkgs: map[string]*pkg{}, types: map[string][]Field{}}
	spec, err := g.build()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	js, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	outputs := map[string][]byte{specPath: append(js, '\n'), jsPath: frontendJS(spec)}
	stale := 0
	for _, p := range []string{specPath, jsPath} {
		if !*check {
			if err := os.WriteFile(p, outputs[p], 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			continue
		}
		if old, _ := os.ReadFile(p); !bytes.Equal(old, outputs[p]) {
			fmt.Printf("%s가 코드와 다릅니다. %s 로 다시 만드세요.\n", p, regenerate)
			stale++
		}
	}
	if stale > 0 {
		os.Exit(1)
	}
	fmt.Println("API 명세 최신")
}

// 루트 패키지를 읽어 메서드·이벤트·오류·노트를 모으고, 닿는 타입을 따라가며 채운다.
func (g *gen) build() (*Spec, error) {
	root, err := g.load(".")
	if err != nil {
		return nil, err
	}
	spec := &Spec{
		Generated: regenerate + " (-check로 검사). 손으로 고치지 말 것",
		Notes:     map[string][]string{}, Errors: map[string]string{},
	}
	consts := map[string]string{}
	var payload string
	emitted := map[string]bool{}
	for _, f := range root.files {
		name := filepath.Base(g.fs.Position(f.Pos()).Filename)
		if notes := headNotes(f); len(notes) > 0 {
			spec.Notes[name] = notes
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				collectConsts(d, consts)
			case *ast.FuncDecl:
				if d.Name.Name == emitter {
					payload = compositeType(d.Body)
				}
				ast.Inspect(d, func(n ast.Node) bool {
					if c, ok := n.(*ast.CallExpr); ok && calls(c, emitter) && len(c.Args) == 1 {
						if id, ok := c.Args[0].(*ast.Ident); ok {
							emitted[id.Name] = true
						}
					}
					return true
				})
				if m, ok := g.method(root, d); ok {
					spec.Methods = append(spec.Methods, m)
				}
			}
		}
	}
	if payload == "" {
		return nil, fmt.Errorf("%s에서 보내는 구조체를 찾지 못함", emitter)
	}
	g.typeName(root, &ast.Ident{Name: payload})
	for name, v := range consts {
		switch {
		case strings.HasPrefix(name, "msg"):
			spec.Errors[name] = v
		case strings.HasPrefix(name, "ev"):
			if !emitted[name] {
				return nil, fmt.Errorf("이벤트 %s가 %s로 보내지지 않음", name, emitter)
			}
			spec.Events = append(spec.Events, Event{Name: v, Const: name, Payload: payload})
		}
	}
	sort.Slice(spec.Methods, func(i, j int) bool { return spec.Methods[i].Name < spec.Methods[j].Name })
	sort.Slice(spec.Events, func(i, j int) bool { return spec.Events[i].Name < spec.Events[j].Name })
	spec.Types = g.types
	return spec, nil
}

// dir의 _test가 아닌 .go 파일을 읽어 구조체와 import 별칭을 모은다. 한 번 읽은 폴더는 다시 쓰지 않는다.
func (g *gen) load(dir string) (*pkg, error) {
	if p, ok := g.pkgs[dir]; ok {
		return p, nil
	}
	p := &pkg{structs: map[string]*ast.StructType{}, imports: map[string]string{}}
	names, err := filepath.Glob(filepath.Join(g.root, dir, "*.go"))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(g.fs, n, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		p.files = append(p.files, f)
		for _, im := range f.Imports {
			path, _ := strconv.Unquote(im.Path.Value)
			alias := filepath.Base(path)
			if im.Name != nil {
				alias = im.Name.Name
			}
			p.imports[alias] = path
		}
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, s := range gd.Specs {
				if ts, ok := s.(*ast.TypeSpec); ok {
					if st, ok := ts.Type.(*ast.StructType); ok {
						p.structs[ts.Name.Name] = st
					}
				}
			}
		}
	}
	g.pkgs[dir] = p
	return p, nil
}

// *App의 공개 메서드만 받고, 결과 중 error는 따로 표시한다.
func (g *gen) method(root *pkg, d *ast.FuncDecl) (Method, bool) {
	if d.Recv == nil || !d.Name.IsExported() || recvName(d.Recv) != "App" {
		return Method{}, false
	}
	m := Method{Name: d.Name.Name, Doc: strings.TrimSpace(d.Doc.Text()), Params: []string{}}
	for _, p := range d.Type.Params.List {
		t := g.typeName(root, p.Type)
		for range max(1, len(p.Names)) {
			m.Params = append(m.Params, t)
		}
	}
	if d.Type.Results != nil {
		for _, r := range d.Type.Results.List {
			if id, ok := r.Type.(*ast.Ident); ok && id.Name == "error" {
				m.Error = true
				continue
			}
			m.Returns = g.typeName(root, r.Type)
		}
	}
	return m, true
}

// Go 타입을 TypeScript 표기로 바꾸고, 구조체를 만나면 types에 필드를 채운다.
func (g *gen) typeName(p *pkg, e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return "string"
		case "bool":
			return "boolean"
		case "int", "int32", "int64", "uint", "float32", "float64":
			return "number"
		}
		if st, ok := p.structs[t.Name]; ok {
			g.addStruct(p, t.Name, st)
			return t.Name
		}
	case *ast.SelectorExpr:
		x, _ := t.X.(*ast.Ident)
		if x != nil && strings.HasPrefix(p.imports[x.Name], module) {
			sub, err := g.load(strings.TrimPrefix(p.imports[x.Name], module))
			if err == nil {
				if st, ok := sub.structs[t.Sel.Name]; ok {
					g.addStruct(sub, t.Sel.Name, st)
					return t.Sel.Name
				}
			}
		}
	case *ast.StarExpr:
		return g.typeName(p, t.X)
	case *ast.ArrayType:
		elem := g.typeName(p, t.Elt)
		if t.Len == nil {
			return elem + "[]"
		}
		n, _ := strconv.Atoi(t.Len.(*ast.BasicLit).Value)
		return "[" + strings.TrimSuffix(strings.Repeat(elem+", ", n), ", ") + "]"
	case *ast.MapType:
		return "Record<" + g.typeName(p, t.Key) + ", " + g.typeName(p, t.Value) + ">"
	}
	panic(fmt.Sprintf("명세로 옮기지 못하는 타입: %s", exprString(e)))
}

// json 태그가 있는 필드만 싣는다. 태그가 "-"면 뺀다.
func (g *gen) addStruct(p *pkg, name string, st *ast.StructType) {
	if _, ok := g.types[name]; ok {
		return
	}
	g.types[name] = nil
	var fields []Field
	for _, f := range st.Fields.List {
		if f.Tag == nil || len(f.Names) == 0 {
			continue
		}
		tag, _ := strconv.Unquote(f.Tag.Value)
		key, _, _ := strings.Cut(reflectTag(tag, "json"), ",")
		if key == "" || key == "-" {
			continue
		}
		fields = append(fields, Field{JSON: key, Go: f.Names[0].Name, Type: g.typeName(p, f.Type)})
	}
	g.types[name] = fields
}

// 파일에서 package 앞에 있는 주석 줄들
func headNotes(f *ast.File) []string {
	var out []string
	for _, c := range f.Comments {
		if c.End() >= f.Package {
			break
		}
		for _, l := range c.List {
			t := strings.TrimSpace(strings.TrimPrefix(l.Text, "//"))
			if t != "" && !strings.HasPrefix(l.Text, "//go:") && !strings.HasPrefix(t, "Package ") {
				out = append(out, t)
			}
		}
	}
	return out
}

// 이름 = "문자열" 꼴 상수만 모은다.
func collectConsts(d *ast.GenDecl, out map[string]string) {
	if d.Tok != token.CONST {
		return
	}
	for _, s := range d.Specs {
		vs := s.(*ast.ValueSpec)
		for i, n := range vs.Names {
			if i < len(vs.Values) {
				if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					out[n.Name], _ = strconv.Unquote(lit.Value)
				}
			}
		}
	}
}

// 함수 본문의 첫 복합 리터럴 타입 이름(progressEmitter가 보내는 구조체)
func compositeType(body *ast.BlockStmt) string {
	var name string
	ast.Inspect(body, func(n ast.Node) bool {
		if cl, ok := n.(*ast.CompositeLit); ok && name == "" {
			if id, ok := cl.Type.(*ast.Ident); ok {
				name = id.Name
			}
		}
		return name == ""
	})
	return name
}

// x.name(...) 호출인지 본다.
func calls(c *ast.CallExpr, name string) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == name
}

func recvName(fl *ast.FieldList) string {
	t := fl.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// reflect.StructTag.Get과 같은 규칙으로 key:"value"를 찾는다.
func reflectTag(tag, key string) string {
	for _, part := range strings.Fields(tag) {
		k, v, ok := strings.Cut(part, ":")
		if ok && k == key {
			s, _ := strconv.Unquote(v)
			return s
		}
	}
	return ""
}

func exprString(e ast.Expr) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%T", e)
	return b.String()
}

// 이벤트는 ev를 뗀 소문자 키, 안내 문장은 상수 이름, 타입은 JSDoc typedef로 싣는다.
func frontendJS(s *Spec) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// %s이 docs/api.json과 함께 만든다. 손으로 고치지 말고 %s 로 다시 만든다.\n\n", "cmd/apispec", regenerate)
	b.WriteString("export const EVENTS = /** @type {const} */ ({\n")
	for _, e := range s.Events {
		key := strings.TrimPrefix(e.Const, "ev")
		fmt.Fprintf(&b, "    %s: %s,\n", strings.ToLower(key[:1])+key[1:], quoteJS(e.Name))
	}
	b.WriteString("});\n\nexport const MESSAGES = {\n")
	keys := make([]string, 0, len(s.Errors))
	for k := range s.Errors {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "    %s: %s,\n", k, quoteJS(s.Errors[k]))
	}
	b.WriteString("};\n")
	names := make([]string, 0, len(s.Types))
	for n := range s.Types {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(&b, "\n/**\n * @typedef {Object} %s\n", n)
		for _, f := range s.Types[n] {
			fmt.Fprintf(&b, " * @property {%s} %s\n", f.Type, f.JSON)
		}
		b.WriteString(" */\n")
	}
	return b.Bytes()
}

// 작은따옴표 JS 문자열. 안의 작은따옴표와 역슬래시만 이스케이프한다.
func quoteJS(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}
