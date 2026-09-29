// 3단계: 자동 가리기, 소리 없애기, 직접 가릴 곳

import {h} from '../ui/dom.js';
import {createPlayer} from '../ui/player.js';
import {createGuide} from '../ui/message.js';
import {createMaskOverlay} from '../mask/overlay.js';
import {openVideo} from '../media/videoSource.js';
import {createAutoPanel} from '../mask/autoPanel.js';
import {createSoundPanel} from '../mask/soundPanel.js';
import {state, MAX_RECTS, autoRectsAt, toggleExcluded} from '../state.js';

export const maskStep = {
    title: '가릴 곳 정하기',
    mount(root, ctx) {
        const v = state.video;
        const guide = createGuide('얼굴은 ', h('b', {}, '"사람 얼굴 가리기"'), ', 그 밖은 ',
            h('b', {}, '마우스로 끌어서'), ' 가리세요. 없으면 ', h('b', {}, '"다음"'), '.');
        const list = h('ol.rect-list');

        const player = createPlayer({
            video: openVideo(v.url), duration: v.duration, min: state.start, max: state.end, fineSteps: false,
        });
        const overlay = createMaskOverlay({
            video: player.video,
            getRects: () => state.rects,
            getAuto: autoRectsAt,
            onAutoClick: (b) => {
                player.pause();
                const removed = toggleExcluded(b.kind, b.id);
                guide.set(removed
                    ? '이 자동 가림을 뺐습니다. 잘못 뺐으면 회색 네모를 다시 누르세요.'
                    : '자동 가림을 되살렸습니다.', 'success');
                overlay.redraw();
            },
            canAdd: () => state.rects.length < MAX_RECTS,
            onStart: () => player.pause(),
            onAdd: (r) => {
                state.rects.push(r);
                renderList();
                guide.set(`${state.rects.length}번 가림을 만들었습니다. 다 했으면 "다음"을 누르세요.`, 'success');
            },
            onBlocked: () => guide.set(`가릴 곳은 ${MAX_RECTS}개까지입니다. 필요 없는 것을 지워 주세요.`, 'error'),
        });
        player.video.parentElement.append(overlay.el);
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
                    h('p.hint', {}, `최대 ${MAX_RECTS}개. 자른 구간 내내 같은 자리가 가려집니다.`),
                    list,
                ),
            ),
        );

        function renderList() {
            if (state.rects.length === 0) {
                list.replaceChildren(h('li.empty', {}, '아직 없습니다'));
            } else {
                list.replaceChildren(...state.rects.map((_, i) =>
                    h('li', {},
                        h('span.badge', {}, String(i + 1)),
                        h('span', {}, `${i + 1}번 가림`),
                        h('button.btn.small.danger', {type: 'button', onclick: () => remove(i)}, '지우기'),
                    )));
            }
            overlay.redraw();
        }

        function remove(i) {
            state.rects.splice(i, 1);
            guide.reset();
            renderList();
        }

        renderList();
        ctx.setNextEnabled(true);
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
