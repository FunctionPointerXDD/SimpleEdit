// 3단계: 자동 가리기, 소리 없애기, 직접 가릴 곳
// 직접 가림은 처음엔 항상 가리고, 영상을 옮겨 놓고 [여기서부터]·[여기까지]로 시간을 줄인다.
// 2단계 "여기서 시작/끝"과 같은 방식이다. 시간을 줄인 가림은 재생 막대 아래 번호 띠로 보이고,
// 미리보기에는 켜진 시각에만 나온다. 목록은 최근에 그린 것이 위다(번호는 그린 순서 그대로).
// 구간 시작·끝에 0.05초 안으로 붙여 누르면 그쪽은 비워(null) 구간을 다시 잘라도 따라가게 한다.
// 구간을 다시 잘라 가림이 구간 밖이 되면 영상을 거기로 옮길 수 없어, 누른 쪽만 새로 정하고 반대쪽은 비운다.

import {h} from '../ui/dom.js';
import {createPlayer} from '../ui/player.js';
import {createGuide} from '../ui/message.js';
import {createMaskOverlay} from '../mask/overlay.js';
import {openVideo} from '../media/videoSource.js';
import {createAutoPanel} from '../mask/autoPanel.js';
import {createSoundPanel} from '../mask/soundPanel.js';
import {state, MAX_RECTS, autoRectsAt, toggleExcluded, rectSpan, rectActiveAt} from '../state.js';
import {formatClock, roundTenth} from '../util/time.js';

const EDGE = 0.05;

export const maskStep = {
    title: '가릴 곳 정하기',
    mount(root, ctx) {
        const v = state.video;
        const guide = createGuide('얼굴은 ', h('b', {}, '"사람 얼굴 가리기"'), ', 그 밖은 ',
            h('b', {}, '마우스로 끌어서'), ' 가리세요. 없으면 ', h('b', {}, '"다음"'), '.');
        const list = h('ol.rect-list');
        const spans = h('div.rect-spans');

        const player = createPlayer({
            video: openVideo(v.url), duration: v.duration, min: state.start, max: state.end, fineSteps: false,
        });
        const overlay = createMaskOverlay({
            video: player.video,
            getRects: (t) => state.rects.flatMap((r, i) => (rectActiveAt(r, t) ? [{...r, label: i + 1}] : [])),
            getAuto: autoRectsAt,
            onAutoClick: (b) => {
                player.pause();
                const removed = toggleExcluded(b.kind, b.id);
                guide.set(removed
                    ? '이 자동 가림을 뺐습니다. 잘못 뺐으면 회색 동그라미를 다시 누르세요.'
                    : '자동 가림을 되살렸습니다.', 'success');
                overlay.redraw();
            },
            canAdd: () => state.rects.length < MAX_RECTS,
            onStart: () => player.pause(),
            onAdd: (r) => {
                state.rects.push({...r, from: null, to: null});
                renderList();
                guide.set(`${state.rects.length}번 가림을 만들었습니다. 항상 가립니다. `
                    + '일부 시간만 가리려면 오른쪽 목록 맨 위의 "여기서부터"·"여기까지"를 누르세요.', 'success');
            },
            onBlocked: () => guide.set(`가릴 곳은 ${MAX_RECTS}개까지입니다. 필요 없는 것을 지워 주세요.`, 'error'),
        });
        player.video.parentElement.append(overlay.el);
        player.marks.append(spans);
        player.seek(state.start);
        const auto = createAutoPanel({ctx, guide, player, onChange: () => overlay.redraw()});
        const sound = createSoundPanel({guide, video: player.video});

        root.append(
            h('section.panel.editor.with-side', {},
                h('div.editor-main', {}, guide.el, player.el),
                h('aside.side', {},
                    auto.el,
                    sound.el,
                    h('h2', {}, '직접 가릴 곳'),
                    h('p.hint', {}, `최대 ${MAX_RECTS}개. 처음엔 항상 가립니다. 최근에 그린 것이 맨 위입니다. `
                        + '일부 시간만 가리려면 영상을 그 시각으로 옮기고 "여기서부터"·"여기까지"를 누르세요.'),
                    list,
                ),
            ),
        );

        function renderList() {
            if (state.rects.length === 0) {
                list.replaceChildren(h('li.empty', {}, '아직 없습니다'));
            } else {
                list.replaceChildren(...state.rects.map((r, i) => {
                    const span = rectSpan(r);
                    const whole = r.from === null && r.to === null;
                    const time = !span ? '자른 구간 밖이라 가리지 않습니다'
                        : whole ? '항상 가림' : `${formatClock(span[0])} ~ ${formatClock(span[1])}`;
                    const btn = (label, onclick, disabled = false) =>
                        h('button.btn.small', {type: 'button', onclick, disabled}, label);
                    return h('li.rect-item', {},
                        h('div.rect-head', {},
                            h('span.badge', {}, String(i + 1)),
                            h('span', {}, `${i + 1}번 가림`),
                            h('button.btn.small.danger', {type: 'button', onclick: () => remove(i)}, '지우기'),
                        ),
                        h(span ? 'div.rect-time' : 'div.rect-time.out', {}, time,
                            span && !whole && h('button.link', {type: 'button', onclick: () => player.playRange(span[0], span[1])}, '이 부분 보기')),
                        h('div.rect-actions', {},
                            btn('여기서부터', () => setFrom(i)),
                            btn('여기까지', () => setTo(i)),
                            btn('항상 가리기', () => setWhole(i), whole),
                        ),
                    );
                }).reverse());
            }
            renderSpans();
            overlay.redraw();
        }

        function renderSpans() {
            const len = state.end - state.start || 1;
            spans.replaceChildren(...state.rects.flatMap((r, i) => {
                const span = rectSpan(r);
                if (!span || (r.from === null && r.to === null)) return [];
                const left = (span[0] - state.start) / len * 100;
                const width = (span[1] - span[0]) / len * 100;
                return [h('div.rect-span', {style: `left:${left}%;width:${width}%`}, String(i + 1))];
            }));
        }

        function setFrom(i) {
            const r = state.rects[i];
            const t = roundTenth(player.currentTime);
            if (!rectSpan(r)) r.to = null;
            if (t >= (r.to ?? state.end) - EDGE) {
                guide.set(`${i + 1}번 가림의 끝(${formatClock(r.to ?? state.end)})보다 앞에서 눌러 주세요.`, 'error');
                return;
            }
            r.from = t <= state.start + EDGE ? null : t;
            guide.set(`${i + 1}번 가림을 ${formatClock(t)}부터 가립니다.`, 'success');
            renderList();
        }

        function setTo(i) {
            const r = state.rects[i];
            const t = roundTenth(player.currentTime);
            if (!rectSpan(r)) r.from = null;
            if (t <= (r.from ?? state.start) + EDGE) {
                guide.set(`${i + 1}번 가림의 시작(${formatClock(r.from ?? state.start)})보다 뒤에서 눌러 주세요.`, 'error');
                return;
            }
            r.to = t >= state.end - EDGE ? null : t;
            guide.set(`${i + 1}번 가림을 ${formatClock(t)}까지 가립니다.`, 'success');
            renderList();
        }

        function setWhole(i) {
            state.rects[i].from = state.rects[i].to = null;
            guide.set(`${i + 1}번 가림을 항상 가립니다.`, 'success');
            renderList();
        }

        function remove(i) {
            state.rects.splice(i, 1);
            guide.reset();
            renderList();
        }

        renderList();
        ctx.setNextEnabled(true);
        auto.resume();
        return {
            unmount() {
                auto.unmount();
                sound.unmount();
                overlay.destroy();
                overlay.el.remove();
                player.destroy();
            },
        };
    },
};
