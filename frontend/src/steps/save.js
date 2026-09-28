import {h} from '../ui/dom.js';
import {createMessage} from '../ui/message.js';
import {createProgress} from '../ui/progress.js';
import * as api from '../api.js';
import {state, resetAll, enabledAuto} from '../state.js';
import {formatClock, formatLength} from '../util/time.js';

const STAGE_TEXT = {export: '영상을 저장하는 중입니다…'};

// 4단계: 이름·위치를 정해 MP4로 저장
export const saveStep = {
    title: '저장하기',
    lastStep: true,
    mount(root, ctx) {
        const msg = createMessage();
        const progress = createProgress({stageText: STAGE_TEXT, onCancel: () => api.cancel()});
        const saveBtn = h('button.btn.primary.huge', {type: 'button', onclick: save}, '저장하기');
        const folderText = h('span.folder-path', {}, '…');
        const folderBtn = h('button.btn.secondary.small', {type: 'button', onclick: chooseFolder}, '저장 위치 바꾸기');
        const nameInput = h('input.name-input', {
            type: 'text', spellcheck: false, 'aria-label': '파일 이름',
            oninput: () => { state.fileName = nameInput.value; },
            onkeydown: (e) => { if (e.key === 'Enter' && !saveBtn.hidden) save(); },
        });
        const done = h('div.done-card', {hidden: true});
        const len = state.end - state.start;

        root.append(
            h('section.panel', {},
                h('h1', {}, '영상을 저장합니다'),
                h('dl.summary', {},
                    h('dt', {}, '자른 구간'), h('dd', {}, `${formatClock(state.start)} ~ ${formatClock(state.end)} (${formatLength(len)})`),
                    h('dt', {}, '자동 가림'), h('dd', {}, enabledAuto().length ? '사람 얼굴' : '없음'),
                    h('dt', {}, '직접 가린 곳'), h('dd', {}, state.rects.length ? `${state.rects.length}곳` : '없음'),
                    h('dt', {}, '파일 이름'), h('dd.name-row', {}, nameInput, h('span.ext', {}, '.mp4')),
                    h('dt', {}, '저장 위치'), h('dd.folder-row', {}, folderText, folderBtn),
                ),
                h('div.action-row', {}, saveBtn),
                progress.el,
                msg.el,
                done,
            ),
        );

        let off = null;

        async function fillDefaults() {
            const [folder, name] = await Promise.all([api.outputFolder(), api.defaultFileName()]);
            folderText.textContent = folder;
            if (!state.fileName) state.fileName = name;
            nameInput.value = state.fileName;
        }

        async function chooseFolder() {
            folderBtn.disabled = true;
            try {
                folderText.textContent = await api.chooseOutputFolder();
            } finally {
                folderBtn.disabled = false;
            }
        }

        async function save() {
            msg.clear();
            done.hidden = true;
            saveBtn.hidden = true;
            folderBtn.disabled = true;
            nameInput.disabled = true;
            ctx.setBusy(true);
            progress.start('export');
            off = api.onProgress('export', (p) => progress.update(p));
            try {
                state.savedPath = await api.exportVideo({
                    start: state.start, end: state.end, rects: state.rects, fileName: state.fileName,
                    auto: enabledAuto(),
                });
                showDone();
            } catch (e) {
                const text = api.errorText(e);
                msg.set(text, text === api.CANCELED_MESSAGE ? 'info' : 'error');
                saveBtn.hidden = false;
            } finally {
                off?.();
                off = null;
                progress.hide();
                folderBtn.disabled = false;
                nameInput.disabled = false;
                ctx.setBusy(false);
            }
        }

        function showDone() {
            const parts = state.savedPath.split(/[\\/]/);
            const name = parts.pop();
            done.replaceChildren(
                h('div.done-title', {}, '✔ 저장했습니다'),
                h('p', {}, h('b', {}, parts.join('\\')), ' 폴더에 ', h('b', {}, name), ' 파일로 저장했습니다.'),
                h('div.done-actions', {},
                    h('button.btn.primary', {type: 'button', onclick: () => api.openOutputFolder()}, '저장 폴더 열기'),
                    h('button.btn.secondary', {type: 'button', onclick: startOver}, '새 영상 편집하기'),
                ),
            );
            done.hidden = false;
        }

        function startOver() {
            api.closeVideo();
            resetAll();
            ctx.goTo(0);
        }

        fillDefaults();
        return {unmount: () => off?.()};
    },
};
