// JS 주석 규칙 검사: 함수 안 금지, 한 묶음 50자 이내
import fs from 'node:fs';
import path from 'node:path';
import * as acorn from 'acorn';
import * as walk from 'acorn-walk';

const MAX_CHARS = 50;
const root = process.argv[2] || '.';

function listJS(dir) {
    return fs.readdirSync(dir, {withFileTypes: true}).flatMap((e) => {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) return listJS(p);
        return p.endsWith('.js') ? [p] : [];
    });
}

function groupComments(comments) {
    const groups = [];
    for (const c of comments) {
        const last = groups.at(-1);
        if (last && c.type === 'Line' && c.loc.start.line === last.endLine + 1) {
            last.text += c.value.trim();
            last.endLine = c.loc.end.line;
        } else {
            groups.push({start: c.start, line: c.loc.start.line, endLine: c.loc.end.line,
                text: c.value.replace(/^\*|\n\s*\*/g, ' ').trim()});
        }
    }
    return groups;
}

function checkFile(file) {
    const comments = [];
    const ast = acorn.parse(fs.readFileSync(file, 'utf8'),
        {ecmaVersion: 'latest', sourceType: 'module', onComment: comments, locations: true});
    const bodies = [];
    walk.full(ast, (n) => {
        if (/Function/.test(n.type)) bodies.push([n.body.start, n.body.end]);
    });
    let bad = 0;
    for (const g of groupComments(comments)) {
        if (bodies.some(([a, b]) => g.start > a && g.start < b)) {
            console.log(`${file}:${g.line}: 함수 안 주석: ${g.text}`);
            bad++;
        }
        const n = [...g.text].length;
        if (n > MAX_CHARS) {
            console.log(`${file}:${g.line}: ${n}자: ${g.text}`);
            bad++;
        }
    }
    return bad;
}

const bad = listJS(root).reduce((sum, f) => sum + checkFile(f), 0);
console.log('위반', bad);
process.exit(bad ? 1 : 0);
