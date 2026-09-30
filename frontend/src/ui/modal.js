// 확인 창은 화면 전체를 덮어 뒤를 누르지 못하게 하고, 두 버튼 중 하나를 고를 때까지 기다린다.
// 처음 초점은 안전한 쪽(취소) 버튼. Esc와 창 바깥 누르기도 취소로 본다.

import {h} from './dom.js';

// 창을 붙이고 Tab은 두 버튼 사이만 돌게 한다. 닫히면 원래 초점으로 돌려 놓고 true/false로 끝난다.
/**
 * @param {{title: string, lines: string[], confirmText: string, cancelText: string}} opts
 * @returns {Promise<boolean>}
 */
export function confirmDialog({title, lines, confirmText, cancelText}) {
    return new Promise((resolve) => {
        const before = /** @type {HTMLElement | null} */ (document.activeElement);
        const cancelBtn = h('button.btn.secondary.big', {type: 'button', onclick: () => close(false)}, cancelText);
        const okBtn = h('button.btn.primary.big', {type: 'button', onclick: () => close(true)}, confirmText);
        const dialog = h('div.modal', {role: 'dialog', 'aria-modal': 'true', 'aria-labelledby': 'modal-title'},
            h('h2.modal-title#modal-title', {}, title),
            ...lines.map((l) => h('p.modal-line', {}, l)),
            h('div.modal-actions', {}, cancelBtn, okBtn),
        );
        const backdrop = h('div.modal-backdrop', {}, dialog);
        backdrop.addEventListener('click', (e) => {
            if (e.target === backdrop) close(false);
        });

        function onKey(e) {
            if (e.key === 'Escape') {
                e.preventDefault();
                close(false);
            } else if (e.key === 'Tab') {
                e.preventDefault();
                (document.activeElement === cancelBtn ? okBtn : cancelBtn).focus();
            }
        }

        function close(ok) {
            document.removeEventListener('keydown', onKey, true);
            backdrop.remove();
            before?.focus?.();
            resolve(ok);
        }

        document.addEventListener('keydown', onKey, true);
        document.body.append(backdrop);
        cancelBtn.focus();
    });
}
