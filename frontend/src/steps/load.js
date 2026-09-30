// 1단계: 유튜브 주소로 받기, 최근 받은 영상 다시 열기, 또는 내 영상 열기
// 최근 받은 영상 버튼은 Go가 7일 보관하는 원본 전부를 목록 창으로 보여 주고, 고른 것을 다시 받지 않고 연다.

import {h} from '../ui/dom.js';
import {createMessage} from '../ui/message.js';
import {createProgress} from '../ui/progress.js';
import * as api from '../api.js';
import {openVideo} from '../media/videoSource.js';
import {state, setVideo} from '../state.js';
import {formatLength, formatDay, formatWhole} from '../util/time.js';
import {pickDialog} from '../ui/modal.js';

const STAGE_TEXT = {
    start: '영상 정보를 확인하는 중입니다…',
    download: '영상을 받는 중입니다…',
    merge: '거의 다 됐습니다. 마무리하는 중입니다…',
    convert: '미리보기를 준비하는 중입니다…',
    retry: '유튜브 연결이 끊겨 다시 받는 중입니다. 잠시만 기다려 주세요…',
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
        const recentBtn = h('button.btn.secondary', {type: 'button', onclick: openRecentList}, '최근 받은 영상');

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
                h('p', {}, '전에 받은 영상이나 내 컴퓨터에 있는 영상을 쓰려면'),
                recentBtn,
                localBtn,
            ),
        );

        const controls = [getBtn, recentBtn, localBtn, urlInput];
        const reveal = (el) => requestAnimationFrame(() => el.scrollIntoView({block: 'nearest'}));
        let off = null;

        function setWorking(working) {
            controls.forEach((c) => (c.disabled = working));
            ctx.setBusy(working);
        }

        async function openRecentList() {
            const items = await api.recentVideos();
            const id = await pickDialog({
                title: '최근 받은 영상',
                lines: ['받은 영상은 7일 동안 보관합니다. 고르면 다시 받지 않고 바로 엽니다.'],
                empty: '7일 안에 받은 영상이 없습니다. 유튜브 주소로 받아 주세요.',
                closeText: '닫기',
                items: items.map((it) => ({
                    label: it.title,
                    sub: `길이 ${formatWhole(it.duration)} · ${formatDay(it.savedAt)} 받음`,
                    value: it.id,
                })),
            });
            if (id) run(() => api.openRecent(id), true);
        }

        function showLoaded() {
            const v = state.video;
            loaded.hidden = !v;
            ctx.setNextEnabled(!!v);
            if (!v) return;
            loaded.replaceChildren(
                h('div.loaded-title', {}, '✔ 영상을 불러왔습니다'),
                h('div', {}, `${v.name} · 길이 ${formatLength(v.duration)} · ${v.width}×${v.height}`),
                v.reused && h('div', {}, '전에 받아 둔 파일이라 다시 받지 않았습니다.'),
                v.lowered && h('div', {}, '유튜브 서버 문제로 고화질(1080p)을 받지 못해 720p로 받았습니다.'),
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
            off = api.onProgress(api.EVENTS.downloadProgress, (p) => {
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
