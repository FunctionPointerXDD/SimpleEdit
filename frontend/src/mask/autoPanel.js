// 종류마다 분석 버튼과 켜기/끄기 버튼이 있다. 한 번에 한 종류만 분석하고, 분석 중엔 모두 잠근다.
// 번호판은 모델이 무거워 자른 구간 길이만큼 걸린다는 안내를 버튼 아래에 둔다.
// 얼굴은 주황 동그라미, 번호판은 주황 네모로 미리보기에 그린다(overlay.js).
// resume은 켜졌는데 자른 구간을 다 덮지 못한 종류를 하나씩 차례로 이어서 분석한다.

import {h} from '../ui/dom.js';
import {createProgress} from '../ui/progress.js';
import * as api from '../api.js';
import {state, autoCovers} from '../state.js';

const STAGE_TEXT = {analyze_face: '얼굴을 찾는 중입니다…', analyze_plate: '번호판을 찾는 중입니다…'};

const KINDS = {
    face: {label: '사람 얼굴 가리기', again: '얼굴 다시 찾기', name: '얼굴', mark: '주황 동그라미',
        hint: '버튼을 누르면 자른 구간에서 얼굴을 찾아 가립니다.'},
    plate: {label: '번호판 가리기', again: '번호판 다시 찾기', name: '번호판', mark: '주황 네모',
        hint: '버튼을 누르면 번호판을 찾아 가립니다. 자른 구간 길이만큼 시간이 걸립니다.'},
};

// 종류마다 버튼·켜기 버튼·안내를 만들고, resume은 켜진 종류의 빠진 부분만 이어 붙인다.
export function createAutoPanel({ctx, guide, player, onChange}) {
    const progress = createProgress({stageText: STAGE_TEXT, onCancel: () => api.cancel()});
    const parts = Object.fromEntries(Object.keys(KINDS).map((kind) => [kind, {
        btn: h(`button.btn.${kind === 'face' ? 'primary' : 'secondary'}.auto-btn`, {type: 'button', onclick: () => run(kind, true)}),
        toggle: h('button.btn.secondary.small.auto-toggle', {type: 'button', onclick: () => toggle(kind)}),
        status: h('p.auto-status'),
    }]));
    const el = h('div.auto-panel', {},
        h('h2', {}, '자동으로 가리기'),
        ...Object.values(parts).flatMap((p) => [p.btn, p.toggle, p.status]),
        progress.el);
    let off = null;

    function render() {
        for (const [kind, k] of Object.entries(KINDS)) {
            const a = state.auto[kind];
            const p = parts[kind];
            p.btn.textContent = a ? k.again : k.label;
            p.toggle.hidden = !a;
            if (a) p.toggle.textContent = `${k.name} 가리기 ${a.enabled ? '끄기' : '켜기'}`;
            p.status.textContent = a ? statusText(kind, a) : k.hint;
        }
    }

    /**
     * @param {string} kind
     * @param {import('../state.js').AutoState} a
     */
    function statusText(kind, a) {
        const k = KINDS[kind];
        if (!a.enabled) return `${k.name} 가리기를 꺼 두었습니다.`;
        if (a.result.tracks === 0) return `${k.name}을 찾지 못했습니다. 필요하면 네모를 직접 그리세요.`;
        return `찾은 ${k.name}을 ${k.mark}로 표시했습니다. 잘못 가린 곳은 ${k.mark}를 눌러 빼세요.`;
    }

    function lock(busy) {
        ctx.setBusy(busy);
        for (const p of Object.values(parts)) p.btn.disabled = p.toggle.disabled = busy;
    }

    async function run(kind, fresh) {
        const prev = state.auto[kind];
        const k = KINDS[kind];
        player.pause();
        lock(true);
        progress.start(`analyze_${kind}`);
        off = api.onProgress(api.EVENTS.analyzeProgress, (p) => progress.update(p));
        try {
            const result = await api.analyze({kind, start: state.start, end: state.end, fresh});
            const keep = !fresh && prev;
            state.auto[kind] = {result, enabled: keep ? prev.enabled : true, excluded: keep ? prev.excluded : []};
            if (keep) guide.set(`늘어난 부분까지 ${k.name}을 찾았습니다. 빼 둔 곳은 그대로입니다.`, 'success');
            else guide.set(result.tracks > 0
                ? `${k.name}을 찾았습니다. 재생해서 ${k.mark}가 잘 따라가는지 보세요.`
                : `${k.name}을 찾지 못했습니다. 필요하면 네모를 직접 그리세요.`,
            result.tracks > 0 ? 'success' : 'info');
        } catch (e) {
            const text = api.errorText(e);
            guide.set(text, text === api.CANCELED_MESSAGE ? 'info' : 'error');
        } finally {
            off?.();
            off = null;
            progress.hide();
            lock(false);
            render();
            onChange();
        }
    }

    function toggle(kind) {
        const a = state.auto[kind];
        if (!a) return;
        a.enabled = !a.enabled;
        render();
        onChange();
        if (a.enabled) resume();
    }

    async function resume() {
        for (const [kind, k] of Object.entries(KINDS)) {
            if (state.auto[kind]?.enabled && !autoCovers(kind)) {
                guide.set(`자른 구간이 바뀌어 늘어난 부분의 ${k.name}을 찾습니다.`, 'info');
                await run(kind, false);
            }
        }
    }

    render();
    return {el, resume, unmount: () => off?.()};
}
