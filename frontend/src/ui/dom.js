// h('button.primary', {onclick}, '저장') 형태의 DOM 헬퍼
export function h(tag, props = {}, ...children) {
    const [name, ...rest] = tag.split(/(?=[.#])/);
    const el = document.createElement(name || 'div');
    for (const part of rest) {
        if (part[0] === '.') el.classList.add(part.slice(1));
        else if (part[0] === '#') el.id = part.slice(1);
    }
    for (const [k, v] of Object.entries(props || {})) {
        if (v == null || v === false) continue;
        if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
        else if (k === 'class') el.classList.add(...String(v).split(' ').filter(Boolean));
        else if (k === 'style' && typeof v === 'object') Object.assign(el.style, v);
        else if (k in el && typeof v !== 'string') el[k] = v;
        else el.setAttribute(k, v === true ? '' : v);
    }
    for (const c of children.flat()) {
        if (c == null || c === false) continue;
        el.append(c instanceof Node ? c : document.createTextNode(String(c)));
    }
    return el;
}
