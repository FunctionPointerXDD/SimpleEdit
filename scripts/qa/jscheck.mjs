// JS 주석 규칙 (cmd/cmtcheck와 같음)
// - 파일 맨 위(첫 문장 앞)의 주석은 줄마다 80자 이내
// - 최상위 함수 바로 위 주석은 한 묶음 80자 이내, 동작 방식만 적는다
// - 그 밖의 주석(함수 안, 상수·객체 위)은 위반
// - JSDoc 타입 주석(/** @param … */ 등 @태그가 있는 블록)은 코드로 보고 검사하지 않는다
// LIST=1이면 위반 대신 모든 주석 묶음과 종류를 출력한다.
import fs from 'node:fs';
import path from 'node:path';
import * as acorn from 'acorn';

const MAX_CHARS = 80;
const roots = process.argv.length > 2 ? process.argv.slice(2) : ['.'];
const listMode = process.env.LIST === '1';

function listJS(dir) {
    return fs.readdirSync(dir, {withFileTypes: true}).flatMap((e) => {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) return e.name === 'node_modules' ? [] : listJS(p);
        return p.endsWith('.js') || p.endsWith('.mjs') ? [p] : [];
    });
}

// 이어진 줄 주석은 한 묶음으로, 줄 목록과 합친 글자를 함께 둔다.
function groupComments(comments) {
    const groups = [];
    for (const c of comments) {
        const last = groups.at(-1);
        const lines = c.value.split('\n').map((l) => l.replace(/^\s*\*?/, '').trim()).filter(Boolean);
        if (last && c.type === 'Line' && last.type === 'Line' && c.loc.start.line === last.endLine + 1) {
            last.lines.push(...lines);
            last.endLine = c.loc.end.line;
        } else {
            groups.push({type: c.type, start: c.start, line: c.loc.start.line, endLine: c.loc.end.line, lines});
        }
    }
    return groups;
}

const isFn = (n) => n && /^(Arrow)?Function(Expression|Declaration)$/.test(n.type);

// 최상위 문장 중 함수 선언·함수를 담은 const의 시작 줄
function functionLines(ast) {
    const out = new Set();
    for (const st of ast.body) {
        const d = st.type.startsWith('Export') && st.declaration ? st.declaration : st;
        if (isFn(d) || (d.type === 'VariableDeclaration' && d.declarations.every((v) => isFn(v.init)))) {
            out.add(st.loc.start.line);
        }
    }
    return out;
}

const isJSDocType = (c) => c.type === 'Block' && c.value.startsWith('*') && /@\w+/.test(c.value);

function checkFile(file) {
    const comments = [];
    const ast = acorn.parse(fs.readFileSync(file, 'utf8'),
        {ecmaVersion: 'latest', sourceType: 'module', onComment: comments, locations: true});
    const fnLines = functionLines(ast);
    const jsdocEnd = new Map(comments.filter(isJSDocType).map((c) => [c.loc.start.line, c.loc.end.line]));
    const firstStart = ast.body[0]?.start ?? Infinity;
    let bad = 0;
    for (const g of groupComments(comments.filter((c) => !isJSDocType(c)))) {
        let next = g.endLine + 1;
        while (jsdocEnd.has(next)) next = jsdocEnd.get(next) + 1;
        const kind = g.start < firstStart ? 'HEAD' : fnLines.has(next) ? 'FUNC' : 'OTHER';
        if (listMode) {
            console.log(`${file}:${g.line} ${kind} ${g.lines.join(' | ')}`);
            continue;
        }
        const report = (why, text) => {
            console.log(`${file}:${g.line}: ${why}: ${text}`);
            bad++;
        };
        if (kind === 'HEAD') {
            for (const l of g.lines) if ([...l].length > MAX_CHARS) report(`머리 주석 ${[...l].length}자`, l);
        } else if (kind === 'FUNC') {
            const text = g.lines.join(' ');
            if ([...text].length > MAX_CHARS) report(`함수 주석 ${[...text].length}자`, text);
        } else {
            report('함수 설명 외 주석은 파일 맨 위로', g.lines.join(' '));
        }
    }
    return bad;
}

const files = roots.flatMap((r) => (fs.statSync(r).isDirectory() ? listJS(r) : [r]));
const bad = files.reduce((sum, f) => sum + checkFile(f), 0);
if (!listMode) console.log('위반', bad);
process.exit(bad ? 1 : 0);
