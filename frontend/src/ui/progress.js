import {h} from './dom.js';
import {formatRemaining} from '../util/time.js';

// 진행률 막대, 퍼센트, 남은 시간, 취소 버튼
export function createProgress({stageText, onCancel}) {
    const label = h('div.progress-label');
    const fill = h('div.progress-fill');
    const bar = h('div.progress-bar', {role: 'progressbar', 'aria-valuemin': 0, 'aria-valuemax': 100}, fill);
    const percent = h('span.progress-percent', {}, '0%');
    const remaining = h('span.progress-remaining');
    const cancelBtn = onCancel
        ? h('button.btn.secondary', {type: 'button', onclick: cancel}, '취소')
        : null;
    const el = h('div.progress', {hidden: true},
        label, bar,
        h('div.progress-row', {}, percent, remaining, cancelBtn),
    );

    function cancel() {
        cancelBtn.disabled = true;
        cancelBtn.textContent = '취소하는 중…';
        onCancel();
    }

    function update(p) {
        const pct = Math.max(0, Math.min(100, p.percent));
        fill.style.width = `${pct}%`;
        bar.setAttribute('aria-valuenow', String(Math.round(pct)));
        percent.textContent = `${Math.floor(pct)}%`;
        label.textContent = stageText[p.stage] ?? label.textContent;
        remaining.textContent = `남은 시간: ${formatRemaining(p.remaining)}`;
    }

    function start(stage) {
        el.hidden = false;
        if (cancelBtn) {
            cancelBtn.disabled = false;
            cancelBtn.textContent = '취소';
        }
        update({percent: 0, remaining: -1, stage});
    }

    return {el, start, update, hide: () => { el.hidden = true; }};
}
