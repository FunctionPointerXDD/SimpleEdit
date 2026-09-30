import {h} from '../ui/dom.js';
import {createProgress} from '../ui/progress.js';
import * as api from '../api.js';
import {state, autoCovers} from '../state.js';

const STAGE_TEXT = {analyze: '얼굴을 찾는 중입니다…'};

const KINDS = {
    face: {label: '사람 얼굴 가리기', again: '얼굴 다시 찾기', name: '얼굴'},
};

// 분석 중엔 버튼·토글·다음을 잠근다. 버튼은 새로 분석, resume은 켜진 종류의 빠진 부분만 이어 붙인다.
export function createAutoPanel({ctx, guide, player, onChange}) {
    const progress = createProgress({stageText: STAGE_TEXT, onCancel: () => api.cancel()});
    const faceBtn = h('button.btn.primary.auto-btn', {type: 'button', onclick: () => run('face', true)});
    const faceToggle = h('button.btn.secondary.small.auto-toggle', {type: 'button', onclick: () => toggle('face')});
    const plateBtn = h('button.btn.secondary.auto-btn', {type: 'button', disabled: true}, '번호판 가리기 (준비 중)');
    const status = h('p.auto-status');
    const el = h('div.auto-panel', {},
        h('h2', {}, '자동으로 가리기'),
        faceBtn, faceToggle, plateBtn, status, progress.el);
    let off = null;

    function render() {
        const a = state.auto.face;
        faceBtn.textContent = a ? KINDS.face.again : KINDS.face.label;
        faceToggle.hidden = !a;
        if (a) faceToggle.textContent = a.enabled ? '얼굴 가리기 끄기' : '얼굴 가리기 켜기';
        status.textContent = a ? statusText(a) : '버튼을 누르면 자른 구간에서 얼굴을 찾아 가립니다.';
    }

    /** @param {import('../state.js').AutoState} a */
    function statusText(a) {
        if (!a.enabled) return '얼굴 가리기를 꺼 두었습니다.';
        if (a.result.tracks === 0) return '얼굴을 찾지 못했습니다. 필요하면 네모를 직접 그리세요.';
        return '찾은 얼굴을 주황 동그라미로 표시했습니다. 잘못 가린 곳은 주황 동그라미를 눌러 빼세요.';
    }

    async function run(kind, fresh) {
        const prev = state.auto[kind];
        player.pause();
        ctx.setBusy(true);
        faceBtn.disabled = true;
        faceToggle.disabled = true;
        progress.start('analyze');
        off = api.onProgress(api.EVENTS.analyzeProgress, (p) => progress.update(p));
        try {
            const result = await api.analyze({kind, start: state.start, end: state.end, fresh});
            const keep = !fresh && prev;
            state.auto[kind] = {result, enabled: keep ? prev.enabled : true, excluded: keep ? prev.excluded : []};
            if (keep) guide.set(`늘어난 부분까지 ${KINDS[kind].name}을 찾았습니다. 빼 둔 곳은 그대로입니다.`, 'success');
            else guide.set(result.tracks > 0
                ? `${KINDS[kind].name}을 찾았습니다. 재생해서 주황 동그라미가 잘 따라가는지 보세요.`
                : `${KINDS[kind].name}을 찾지 못했습니다. 필요하면 네모를 직접 그리세요.`,
            result.tracks > 0 ? 'success' : 'info');
        } catch (e) {
            const text = api.errorText(e);
            guide.set(text, text === api.CANCELED_MESSAGE ? 'info' : 'error');
        } finally {
            off?.();
            off = null;
            progress.hide();
            faceBtn.disabled = false;
            faceToggle.disabled = false;
            ctx.setBusy(false);
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

    function resume() {
        for (const kind of Object.keys(KINDS)) {
            if (state.auto[kind]?.enabled && !autoCovers(kind)) {
                guide.set(`자른 구간이 바뀌어 늘어난 부분의 ${KINDS[kind].name}을 찾습니다.`, 'info');
                run(kind, false);
            }
        }
    }

    render();
    return {el, resume, unmount: () => off?.()};
}
