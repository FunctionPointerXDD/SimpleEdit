import {h} from './ui/dom.js';

// 한 단계씩 보이는 마법사. step.mount(root, ctx)
export function createWizard(root, steps) {
    const stepLabel = h('div.step-label');
    const stepDots = h('ol.step-dots', {}, steps.map((s, i) => h('li', {}, `${i + 1}. ${s.title}`)));
    const body = h('main.step-body');
    const prevBtn = h('button.btn.nav', {type: 'button', onclick: () => goTo(current - 1)}, '◀ 이전');
    const nextBtn = h('button.btn.nav.primary', {type: 'button', onclick: () => goTo(current + 1)}, '다음 ▶');

    root.replaceChildren(
        h('header.topbar', {}, stepLabel, stepDots),
        body,
        h('footer.navbar', {}, prevBtn, nextBtn),
    );

    let current = 0;
    let view = null;
    let busy = false;
    let nextEnabled = false;

    function refreshNav() {
        prevBtn.disabled = busy || current === 0;
        prevBtn.style.visibility = current === 0 ? 'hidden' : 'visible';
        nextBtn.hidden = !!steps[current].lastStep;
        nextBtn.disabled = busy || !nextEnabled;
    }

    const ctx = {
        setNextEnabled(enabled) {
            nextEnabled = enabled;
            if (!enabled) nextBtn.classList.remove('pulse');
            refreshNav();
        },
        highlightNext(on) {
            nextBtn.classList.toggle('pulse', on);
        },
        setBusy(b) {
            busy = b;
            refreshNav();
        },
        goTo: (i) => goTo(i),
    };

    function goTo(index) {
        if (busy || index < 0 || index >= steps.length) return;
        view?.unmount?.();
        current = index;
        nextEnabled = false;
        nextBtn.classList.remove('pulse');
        stepLabel.textContent = `${index + 1} / ${steps.length}  ${steps[index].title}`;
        [...stepDots.children].forEach((li, i) => {
            li.classList.toggle('done', i < index);
            li.classList.toggle('current', i === index);
        });
        body.replaceChildren();
        body.scrollTop = 0;
        refreshNav();
        view = steps[index].mount(body, ctx) || null;
    }

    goTo(0);
    return ctx;
}
