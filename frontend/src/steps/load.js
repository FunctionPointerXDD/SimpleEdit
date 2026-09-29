// 1단계: 유튜브 주소로 받기 또는 내 영상 열기

import {h} from '../ui/dom.js';
import {createMessage} from '../ui/message.js';
import {createProgress} from '../ui/progress.js';
import * as api from '../api.js';
import {openVideo} from '../media/videoSource.js';
import {state, setVideo} from '../state.js';
import {formatLength} from '../util/time.js';

const STAGE_TEXT = {
    start: '영상 정보를 확인하는 중입니다…',
    download: '영상을 받는 중입니다…',
    merge: '거의 다 됐습니다. 마무리하는 중입니다…',
    convert: '미리보기를 준비하는 중입니다…',
};

export const loadStep = {
    title: '영상 불러오기',
    mount(root, ctx) {
        const msg = createMessage();
        const progress = createProgress({stageText: STAGE_TEXT, onCancel: () => api.cancel()});
        const urlInput = h('input.url-input', {
            type: 'text', placeholder: '여기에 유튜브 주소가 들어갑니다', spellcheck: false,
            'aria-label': '유튜브 주소',
            onkeydown: (e) => { if (e.key === 'Enter') startDownload(); },
        });
        const getBtn = h('button.btn.primary.big', {type: 'button', onclick: startDownload}, '영상 받기');
        const localBtn = h('button.btn.secondary', {type: 'button', onclick: openLocal}, '내 컴퓨터 영상 열기');
        const loaded = h('div.loaded-card', {hidden: true});

        root.append(
            h('section.panel', {},
                h('h1', {}, '유튜브 주소를 넣어 주세요'),
                h('ol.guide', {},
                    h('li', {}, '유튜브에서 영상 주소를 복사하세요.'),
                    h('li', {}, '아래 칸을 ', h('b', {}, '마우스 오른쪽 버튼'), '으로 누르고 ', h('b', {}, '"붙여넣기"'),
                        '를 고르세요. (Ctrl+V도 됩니다)'),
                    h('li', {}, h('b', {}, '"영상 받기"'), ' 버튼을 누르세요.'),
                ),
                h('div.url-row', {}, urlInput, getBtn),
                progress.el,
                msg.el,
                loaded,
            ),
            h('section.panel.subtle', {},
                h('p', {}, '유튜브 영상이 아니라 내 컴퓨터에 있는 영상을 쓰려면'),
                localBtn,
            ),
        );

        const controls = [getBtn, localBtn, urlInput];
        const reveal = (el) => requestAnimationFrame(() => el.scrollIntoView({block: 'nearest'}));
        let off = null;

        function setWorking(working) {
            controls.forEach((c) => (c.disabled = working));
            ctx.setBusy(working);
        }

        function showLoaded() {
            const v = state.video;
            loaded.hidden = !v;
            ctx.setNextEnabled(!!v);
            if (!v) return;
            loaded.replaceChildren(
                h('div.loaded-title', {}, '✔ 영상을 불러왔습니다'),
                h('div', {}, `${v.name} · 길이 ${formatLength(v.duration)} · ${v.width}×${v.height}`),
                h('div.loaded-hint', {}, '아래 오른쪽의 "다음" 버튼을 눌러 주세요.'),
            );
            ctx.highlightNext(true);
        }

        function showError(e) {
            const text = api.errorText(e);
            msg.set(text, text === api.CANCELED_MESSAGE ? 'info' : 'error');
            reveal(msg.el);
        }

        function accept(v) {
            if (!v?.url) return;
            setVideo(v);
            openVideo(v.url);
        }

        async function run(task, showProgressNow) {
            msg.clear();
            loaded.hidden = true;
            ctx.setNextEnabled(false);
            setWorking(true);
            if (showProgressNow) {
                progress.start('start');
                reveal(progress.el);
            }
            off = api.onProgress('download', (p) => {
                if (progress.el.hidden) {
                    progress.start(p.stage);
                    reveal(progress.el);
                }
                progress.update(p);
            });
            try {
                accept(await task());
            } catch (e) {
                showError(e);
            } finally {
                off?.();
                off = null;
                progress.hide();
                setWorking(false);
                showLoaded();
                if (state.video) reveal(loaded);
            }
        }

        function startDownload() {
            const url = urlInput.value.trim();
            if (!url) {
                showError('주소 칸이 비어 있습니다. 유튜브 주소를 칸에 붙여넣은 뒤 "영상 받기"를 눌러 주세요.');
                urlInput.focus();
                return;
            }
            run(() => api.download(url), true);
        }

        function openLocal() {
            run(() => api.openLocal(), false);
        }

        showLoaded();
        if (!state.video) urlInput.focus();
        return {unmount: () => off?.()};
    },
};
