// 2단계: "여기서 시작"/"여기서 끝"으로 고른 부분을 [이 구간 넣기]로 하나씩 쌓아 이어 붙일 구간을 정한다.
// 넣은 구간이 없으면 고른 부분 하나를 그대로 쓴다(처음엔 영상 전체). 목록 순서가 저장 순서이고
// [위로]·[아래로]로 바꾼다. 겹치거나 맞닿은 구간은 넣을 때 하나로 합친다(state.putSegment).
// [고치기]는 그 구간을 시작·끝 칸에 불러오고, [고친 구간 넣기]가 같은 자리를 바꾼다.
// 구간을 넣은 뒤 새로 고른 부분은 넣기 전까지 쓰지 않는다. 넣지 않은 채면 안내로 알린다.
// 재생 막대에는 넣은 구간을 번호 띠로, 고르는 중인 부분을 옅은 띠로 보인다.

import {h} from '../ui/dom.js';
import {createPlayer} from '../ui/player.js';
import {createGuide} from '../ui/message.js';
import {openVideo} from '../media/videoSource.js';
import {state, checkRange, setRange, putSegment, moveSegment, totalLength, MAX_SEGMENTS} from '../state.js';
import {formatClock, formatLength, roundTenth} from '../util/time.js';

export const trimStep = {
    title: '구간 자르기',
    mount(root, ctx) {
        const v = state.video;
        const player = createPlayer({video: openVideo(v.url), duration: v.duration});
        const guide = createGuide('영상을 보다가 쓰고 싶은 부분의 처음에서 ',
            h('b', {}, '"여기서 시작"'), ', 끝에서 ', h('b', {}, '"여기서 끝"'), '을 누르세요. 여러 부분을 이으려면 ',
            h('b', {}, '"이 구간 넣기"'), '를 누르고 다음 부분을 고르세요.');

        const startVal = h('span.value');
        const endVal = h('span.value');
        const lenVal = h('span.value');
        const selection = h('div.seek-selection');
        const bands = h('div.seg-bands');
        player.marks.append(bands, selection);
        const addBtn = h('button.btn.primary.add-seg', {type: 'button', onclick: add});
        const list = h('ol.seg-list');
        const listTitle = h('h2.seg-title');
        const playAllBtn = h('button.btn.secondary.play-all', {type: 'button', onclick: playAll}, '▶ 이어서 보기');
        const hint = h('p.hint', {}, `최대 ${MAX_SEGMENTS}개. 위에서부터 차례로 이어 붙여 저장합니다.`);
        let touched = state.segments.length === 0;
        let editing = -1;

        const setStart = () => {
            setRange(roundTenth(player.currentTime), state.end);
            touched = true;
            update(true);
        };
        const setEnd = () => {
            setRange(state.start, roundTenth(player.currentTime));
            touched = true;
            update(true);
        };

        const card = (label, value, linkText, onclick) =>
            h('div.mark-card', {},
                h('div', {}, h('span.label', {}, label), value),
                h('button.link', {type: 'button', onclick}, linkText));

        root.append(
            h('section.panel.editor.with-side.trim-editor', {},
                h('div.editor-main', {},
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
                h('aside.side.seg-side', {},
                    listTitle,
                    addBtn,
                    playAllBtn,
                    hint,
                    list,
                ),
            ),
        );

        function add() {
            const check = checkRange();
            if (!check.ok) {
                guide.set(check.message, 'error');
                return;
            }
            if (editing < 0 && state.segments.length >= MAX_SEGMENTS) {
                guide.set(`구간은 ${MAX_SEGMENTS}개까지 넣을 수 있습니다. 필요 없는 구간을 지워 주세요.`, 'error');
                return;
            }
            const replaced = editing >= 0;
            const {index, merged} = putSegment({start: state.start, end: state.end}, editing);
            editing = -1;
            touched = false;
            const joined = merged ? ' 겹치는 구간과 하나로 합쳤습니다.' : '';
            guide.set(`${index + 1}번 구간을 ${replaced ? '고쳤습니다' : '넣었습니다'}(모두 ${formatLength(totalLength())}).${joined} `
                + '더 이을 부분이 있으면 그 처음에서 "여기서 시작"을 누르고, 다 넣었으면 "다음"을 누르세요.', 'success');
            update();
        }

        function edit(i) {
            const s = state.segments[i];
            editing = i;
            touched = false;
            setRange(s.start, s.end);
            player.seek(s.start);
            guide.set(`${i + 1}번 구간을 고칩니다. 시작·끝을 다시 정하고 "고친 구간 넣기"를 누르세요.`, 'info');
            update();
        }

        function stopEdit() {
            editing = -1;
            guide.reset();
            update();
        }

        function move(i, d) {
            if (!moveSegment(i, d)) return;
            if (editing === i) editing = i + d;
            else if (editing === i + d) editing = i;
            guide.set(`${i + 1}번 구간을 ${i + d + 1}번 자리로 옮겼습니다.`, 'success');
            update();
        }

        function remove(i) {
            state.segments.splice(i, 1);
            if (!state.segments.length) touched = true;
            if (editing === i) editing = -1;
            else if (editing > i) editing--;
            guide.set(state.segments.length ? `${i + 1}번 구간을 지웠습니다.` : '넣은 구간을 모두 지웠습니다. 고른 부분 하나를 씁니다.', 'success');
            update();
        }

        function playAll() {
            player.playSequence(state.segments.map((s) => [s.start, s.end]));
        }

        function renderList() {
            const n = state.segments.length;
            listTitle.textContent = n ? `구간 ${n}개 · ${formatLength(totalLength())}` : '넣은 구간 없음';
            hint.hidden = n > 0;
            addBtn.textContent = editing >= 0 ? '고친 구간 넣기' : '＋ 이 구간 넣기';
            playAllBtn.hidden = n < 2;
            if (!n) {
                list.replaceChildren(h('li.empty', {}, '구간을 넣지 않으면 위에서 고른 부분 하나만 씁니다.'));
                return;
            }
            const btn = (label, onclick, disabled = false) => h('button.btn.small', {type: 'button', onclick, disabled}, label);
            list.replaceChildren(...state.segments.map((s, i) => h(editing === i ? 'li.seg-item.editing' : 'li.seg-item', {},
                h('span.badge', {}, String(i + 1)),
                h('div.seg-info', {},
                    h('span.seg-time', {}, `${formatClock(s.start)} ~ ${formatClock(s.end)}`),
                    h('span.seg-len', {}, formatLength(s.end - s.start))),
                h('div.seg-move', {},
                    btn('▲ 위로', () => move(i, -1), i === 0),
                    btn('▼ 아래로', () => move(i, 1), i === n - 1)),
                h('div.seg-actions', {},
                    btn('보기', () => player.playRange(s.start, s.end)),
                    editing === i ? btn('그만', stopEdit) : btn('고치기', () => edit(i)),
                    h('button.btn.small.danger', {type: 'button', onclick: () => remove(i)}, '지우기'),
                ),
            )));
        }

        function renderBands() {
            const d = v.duration || 1;
            bands.replaceChildren(...state.segments.map((s, i) => h(editing === i ? 'div.seg-band.editing' : 'div.seg-band', {
                style: `left:${s.start / d * 100}%;width:${(s.end - s.start) / d * 100}%`,
            }, String(i + 1))));
            const a = Math.max(0, Math.min(state.start, state.end)) / d * 100;
            const b = Math.max(state.start, state.end) / d * 100;
            selection.style.left = `${a}%`;
            selection.style.width = `${b - a}%`;
            selection.hidden = !touched && editing < 0;
        }

        function update(userAction = false) {
            startVal.textContent = formatClock(state.start);
            endVal.textContent = formatClock(state.end);
            const len = state.end - state.start;
            lenVal.textContent = len > 0 ? formatLength(len) : '-';
            const check = checkRange();
            selection.classList.toggle('invalid', !check.ok);
            renderBands();
            renderList();
            const n = state.segments.length;
            const pending = n > 0 && touched && editing < 0;
            addBtn.classList.toggle('pulse', pending && check.ok);
            if (userAction && !check.ok) guide.set(check.message, 'error');
            else if (userAction && pending) {
                guide.set(`${formatLength(len)}를 고르셨습니다. 이어 붙이려면 "이 구간 넣기"를 누르세요. `
                    + '넣지 않고 "다음"을 누르면 넣은 구간만 씁니다.', 'info');
            } else if (userAction && editing >= 0) guide.set('다 고쳤으면 "고친 구간 넣기"를 누르세요.', 'info');
            else if (userAction) {
                guide.set(`좋습니다. ${formatLength(len)}를 고르셨습니다. 다 정했으면 "다음"을, `
                    + '다른 부분도 이으려면 "이 구간 넣기"를 누르세요.', 'success');
            } else if (!n && !check.ok) guide.set(check.message, 'error');
            ctx.setNextEnabled(n > 0 || check.ok);
        }

        update();
        return {unmount: () => player.destroy()};
    },
};
