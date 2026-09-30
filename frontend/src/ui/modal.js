// 창은 화면 전체를 덮어 뒤를 누르지 못하게 하고, 고를 때까지 기다린다. 확인 창과 목록 창이 같은 틀을 쓴다.
// 확인 창의 처음 초점은 안전한 쪽(취소) 버튼, 목록 창은 첫 줄. Esc와 창 바깥 누르기는 취소로 본다.

import {h} from './dom.js';

// 창을 붙이고 Tab은 창 안 버튼만 돌게 한다. 닫히면 원래 초점으로 돌려 놓고 close(값)으로 끝난다.
/**
 * @template T
 * @param {(close: (v: T) => void) => {nodes: Node[], first: HTMLElement, cancelValue: T}} build
 * @returns {Promise<T>}
 */
function showDialog(build) {
    return new Promise((resolve) => {
        const before = /** @type {HTMLElement | null} */ (document.activeElement);
        const {nodes, first, cancelValue} = build(close);
        const dialog = h('div.modal', {role: 'dialog', 'aria-modal': 'true', 'aria-labelledby': 'modal-title'}, ...nodes);
        const backdrop = h('div.modal-backdrop', {}, dialog);
        backdrop.addEventListener('click', (e) => {
            if (e.target === backdrop) close(cancelValue);
        });

        function onKey(e) {
            if (e.key === 'Escape') {
                e.preventDefault();
                close(cancelValue);
            } else if (e.key === 'Tab') {
                e.preventDefault();
                const buttons = [...dialog.querySelectorAll('button')];
                const i = buttons.indexOf(/** @type {HTMLButtonElement} */ (document.activeElement));
                buttons[(i + (e.shiftKey ? buttons.length - 1 : 1)) % buttons.length].focus();
            }
        }

        function close(v) {
            document.removeEventListener('keydown', onKey, true);
            backdrop.remove();
            before?.focus?.();
            resolve(v);
        }

        document.addEventListener('keydown', onKey, true);
        document.body.append(backdrop);
        first.focus();
    });
}

// 취소·확인 두 버튼 중 확인이면 true
/**
 * @param {{title: string, lines: string[], confirmText: string, cancelText: string}} opts
 * @returns {Promise<boolean>}
 */
export function confirmDialog({title, lines, confirmText, cancelText}) {
    return showDialog((close) => {
        const cancelBtn = h('button.btn.secondary.big', {type: 'button', onclick: () => close(false)}, cancelText);
        const okBtn = h('button.btn.primary.big', {type: 'button', onclick: () => close(true)}, confirmText);
        return {
            nodes: [
                h('h2.modal-title#modal-title', {}, title),
                ...lines.map((l) => h('p.modal-line', {}, l)),
                h('div.modal-actions', {}, cancelBtn, okBtn),
            ],
            first: cancelBtn,
            cancelValue: false,
        };
    });
}

// 줄을 누르면 그 값, 닫기·Esc·바깥이면 null. 줄이 없으면 empty 문장과 닫기만 보인다.
/**
 * @template T
 * @param {{title: string, lines: string[], empty: string, closeText: string,
 *   items: {label: string, sub: string, value: T}[]}} opts
 * @returns {Promise<T | null>}
 */
export function pickDialog({title, lines, empty, closeText, items}) {
    return showDialog((close) => {
        const closeBtn = h('button.btn.secondary.big', {type: 'button', onclick: () => close(null)}, closeText);
        const rows = items.map((it) => h('button.btn.secondary.pick-item', {type: 'button', onclick: () => close(it.value)},
            h('span.pick-label', {}, it.label),
            h('span.pick-sub', {}, it.sub)));
        return {
            nodes: [
                h('h2.modal-title#modal-title', {}, title),
                ...lines.map((l) => h('p.modal-line', {}, l)),
                rows.length ? h('div.pick-list', {}, ...rows) : h('p.modal-line.pick-empty', {}, empty),
                h('div.modal-actions', {}, closeBtn),
            ],
            first: rows[0] || closeBtn,
            cancelValue: null,
        };
    });
}
