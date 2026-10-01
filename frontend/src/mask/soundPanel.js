import {h} from '../ui/dom.js';
import {state} from '../state.js';

// state.mute를 뒤집고 미리보기 video.muted를 맞춘다. unmount 때 음소거를 푼다.
export function createSoundPanel({guide, video}) {
    const btn = h('button.btn.secondary.auto-btn.sound-btn', {type: 'button', onclick: toggle});
    const status = h('p.auto-status');
    const el = h('div.auto-panel', {}, h('h2', {}, '소리'), btn, status);

    function render() {
        btn.textContent = state.mute ? '소리 되살리기' : '소리 없애기';
        btn.classList.toggle('on', state.mute);
        status.textContent = state.mute
            ? '저장할 때 소리를 모두 뺍니다. 미리보기도 소리가 꺼졌습니다.'
            : '버튼을 누르면 저장할 영상의 소리를 모두 없앱니다.';
        video.muted = state.mute;
    }

    function toggle() {
        state.mute = !state.mute;
        guide.set(state.mute ? '소리를 없앴습니다. 저장한 영상에는 소리가 없습니다.' : '소리를 되살렸습니다.', 'success');
        render();
    }

    render();
    return {
        el,
        unmount() {
            video.muted = false;
        },
    };
}
