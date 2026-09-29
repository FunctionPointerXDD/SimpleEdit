// 2단계: "여기서 시작"/"여기서 끝"으로 구간 정하기

import {h} from '../ui/dom.js';
import {createPlayer} from '../ui/player.js';
import {createGuide} from '../ui/message.js';
import {openVideo} from '../media/videoSource.js';
import {state, checkRange, setRange} from '../state.js';
import {formatClock, formatLength, roundTenth} from '../util/time.js';

export const trimStep = {
    title: '구간 자르기',
    mount(root, ctx) {
        const v = state.video;
        const player = createPlayer({video: openVideo(v.url), duration: v.duration});
        const guide = createGuide('영상을 보다가 쓰고 싶은 부분의 처음에서 ',
            h('b', {}, '"여기서 시작"'), ', 끝에서 ', h('b', {}, '"여기서 끝"'), '을 누르세요.');

        const startVal = h('span.value');
        const endVal = h('span.value');
        const lenVal = h('span.value');
        const selection = h('div.seek-selection');
        player.marks.append(selection);

        const setStart = () => {
            setRange(roundTenth(player.currentTime), state.end);
            update(true);
        };
        const setEnd = () => {
            setRange(state.start, roundTenth(player.currentTime));
            update(true);
        };

        const card = (label, value, linkText, onclick) =>
            h('div.mark-card', {},
                h('div', {}, h('span.label', {}, label), value),
                h('button.link', {type: 'button', onclick}, linkText));

        root.append(
            h('section.panel.editor', {},
                guide.el,
                player.el,
                h('div.mark-row', {},
                    h('button.btn.mark.start', {type: 'button', onclick: setStart}, '여기서 시작'),
                    h('div.mark-values', {},
                        card('시작', startVal, '시작으로 가기', () => player.seek(state.start)),
                        card('끝', endVal, '끝 부분 보기', () => player.seek(Math.max(0, state.end - 3))),
                        card('길이', lenVal, '고른 부분 보기', () => player.playRange(state.start, state.end)),
                    ),
                    h('button.btn.mark.end', {type: 'button', onclick: setEnd}, '여기서 끝'),
                ),
            ),
        );

        function update(userAction = false) {
            startVal.textContent = formatClock(state.start);
            endVal.textContent = formatClock(state.end);
            const len = state.end - state.start;
            lenVal.textContent = len > 0 ? formatLength(len) : '-';
            const d = v.duration || 1;
            const a = Math.max(0, Math.min(state.start, state.end)) / d * 100;
            const b = Math.max(state.start, state.end) / d * 100;
            selection.style.left = `${a}%`;
            selection.style.width = `${b - a}%`;
            const check = checkRange();
            selection.classList.toggle('invalid', !check.ok);
            if (!check.ok) guide.set(check.message, 'error');
            else if (userAction) guide.set(`좋습니다. ${formatLength(len)}를 고르셨습니다. 다 정했으면 "다음"을 눌러 주세요.`, 'success');
            else guide.reset();
            ctx.setNextEnabled(check.ok);
        }

        update();
        return {unmount: () => player.destroy()};
    },
};
