import {h} from './dom.js';

// 안내 상자. kind: 'info' | 'error' | 'success'
export function createMessage() {
    const el = h('div.message', {role: 'status', 'aria-live': 'polite', hidden: true});
    return {
        el,
        set(text, kind = 'info') {
            el.textContent = text;
            el.className = `message ${kind}`;
            el.hidden = !text;
        },
        clear() {
            el.textContent = '';
            el.hidden = true;
        },
    };
}

// 편집 화면 위 안내 줄. 결과가 나오면 문구가 바뀐다.
export function createGuide(...defaultContent) {
    const el = h('div.guide-line', {role: 'status', 'aria-live': 'polite'});
    const reset = () => {
        el.className = 'guide-line';
        el.replaceChildren(...defaultContent.map((c) => (c instanceof Node ? c.cloneNode(true) : c)));
    };
    reset();
    return {
        el,
        set(text, kind = 'info') {
            el.className = `guide-line ${kind}`;
            el.textContent = text;
        },
        reset,
    };
}
