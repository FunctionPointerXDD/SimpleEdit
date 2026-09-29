import {h} from '../ui/dom.js';
import {createProgress} from '../ui/progress.js';
import * as api from '../api.js';
import {state} from '../state.js';

const STAGE_TEXT = {analyze: '얼굴을 찾는 중입니다…'};

// 자동 가리기 종류별 버튼 문구
const KINDS = {
    face: {label: '사람 얼굴 가리기', again: '얼굴 다시 찾기', name: '얼굴'},
};

// 3단계 오른쪽의 자동 가리기 패널
export function createAutoPanel({ctx, guide, player, onChange}) {
    const progress = createProgress({stageText: STAGE_TEXT, onCancel: () => api.cancel()});
    const faceBtn = h('button.btn.primary.auto-btn', {type: 'button', onclick: () => run('face')});
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

    function statusText(a) {
        if (!a.enabled) return '얼굴 가리기를 꺼 두었습니다.';
        if (a.result.found === 0) return '얼굴을 찾지 못했습니다. 필요하면 네모를 직접 그리세요.';
        return '찾은 얼굴을 주황 네모로 표시했습니다. 잘못 가린 곳은 주황 네모를 눌러 빼세요.';
    }

    async function run(kind) {
        player.pause();
        ctx.setBusy(true);
        faceBtn.disabled = true;
        faceToggle.disabled = true;
        progress.start('analyze');
        off = api.onProgress('analyze', (p) => progress.update(p));
        try {
            const result = await api.analyze({kind, start: state.start, end: state.end});
            state.auto[kind] = {result, enabled: true, excluded: []};
            guide.set(result.found > 0
                ? `${KINDS[kind].name}을 찾았습니다. 재생해서 주황 네모가 잘 따라가는지 보세요.`
                : `${KINDS[kind].name}을 찾지 못했습니다. 필요하면 네모를 직접 그리세요.`,
            result.found > 0 ? 'success' : 'info');
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
    }

    render();
    return {el, unmount: () => off?.()};
}
