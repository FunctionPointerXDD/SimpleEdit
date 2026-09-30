// 4단계: 이름·위치를 정해 MP4로 저장
// 버튼 줄: 저장 전 [저장하기] … [새 영상 편집하기], 저장 후 [저장 폴더 열기][다시 저장하기] … [새 영상 편집하기]
// 저장 후 이전 단계를 다녀와도 완료 카드가 남고, 저장한 뒤 달라진 점을 카드에 알린다.
// 다시 저장: 바뀐 게 없으면 아무 일도 하지 않는다. 이름·폴더가 같으면 확인 창 뒤 같은 파일에 덮어쓴다.
// 이름이나 폴더를 바꿨으면 새 파일로 저장한다. 덮어쓰기 대상 판단은 Go(exportTarget)가 다시 한다.
// 바뀜 여부는 내용(구간·가림·소리)과 대상(이름·폴더)을 JSON으로 저장 때 값과 따로 비교한다.
// 새 영상 편집하기는 저장 안 함 / 저장 후 바뀜 / 저장함 세 경우로 안내를 달리한 확인 창을 띄운다.

import {h} from '../ui/dom.js';
import {createMessage} from '../ui/message.js';
import {createProgress} from '../ui/progress.js';
import {confirmDialog} from '../ui/modal.js';
import * as api from '../api.js';
import {state, resetAll, enabledAuto, autoExclusions} from '../state.js';
import {formatClock, formatLength} from '../util/time.js';

const STAGE_TEXT = {export: '영상을 저장하는 중입니다…'};

export const saveStep = {
    title: '저장하기',
    lastStep: true,
    mount(root, ctx) {
        const msg = createMessage();
        const progress = createProgress({stageText: STAGE_TEXT, onCancel: () => api.cancel()});
        const saveBtn = h('button.btn.primary.huge', {type: 'button', onclick: () => save()}, '저장하기');
        const openBtn = h('button.btn.primary.big', {type: 'button', onclick: () => api.openOutputFolder()}, '저장 폴더 열기');
        const againBtn = h('button.btn.secondary.big', {type: 'button', onclick: saveAgain}, '다시 저장하기');
        const newBtn = h('button.btn.secondary.big', {type: 'button', onclick: startOver}, '새 영상 편집하기');
        const folderText = h('span.folder-path', {}, '…');
        const folderBtn = h('button.btn.secondary.small', {type: 'button', onclick: chooseFolder}, '저장 위치 바꾸기');
        const nameInput = h('input.name-input', {
            type: 'text', spellcheck: false, 'aria-label': '파일 이름',
            oninput: () => {
                state.fileName = nameInput.value;
                render();
            },
            onkeydown: (e) => { if (e.key === 'Enter' && !state.savedPath && !saveBtn.disabled) save(); },
        });
        const done = h('div.done-card', {hidden: true});
        const len = state.end - state.start;

        root.append(
            h('section.panel', {},
                h('h1', {}, '영상을 저장합니다'),
                h('dl.summary', {},
                    h('dt', {}, '자른 구간'), h('dd', {}, `${formatClock(state.start)} ~ ${formatClock(state.end)} (${formatLength(len)})`),
                    h('dt', {}, '자동 가림'), h('dd', {}, enabledAuto().length ? '사람 얼굴' : '없음'),
                    h('dt', {}, '소리'), h('dd', {}, state.mute ? '없앰' : '그대로'),
                    h('dt', {}, '직접 가린 곳'), h('dd', {}, state.rects.length ? `${state.rects.length}곳` : '없음'),
                    h('dt', {}, '파일 이름'), h('dd.name-row', {}, nameInput, h('span.ext', {}, '.mp4')),
                    h('dt', {}, '저장 위치'), h('dd.folder-row', {}, folderText, folderBtn),
                ),
                done,
                h('div.save-actions', {}, h('div.save-actions-main', {}, saveBtn, openBtn, againBtn), newBtn),
                progress.el,
                msg.el,
            ),
        );

        let off = null;

        async function fillDefaults() {
            const [dir, name] = await Promise.all([api.outputFolder(), api.defaultFileName()]);
            folder = folderText.textContent = dir;
            if (!state.fileName) state.fileName = name;
            nameInput.value = state.fileName;
            render();
        }

        async function chooseFolder() {
            folderBtn.disabled = true;
            try {
                folder = folderText.textContent = await api.chooseOutputFolder();
            } finally {
                folderBtn.disabled = false;
                render();
            }
        }

        function content() {
            return {
                start: state.start, end: state.end, rects: state.rects,
                auto: enabledAuto(), mute: state.mute, exclude: autoExclusions(),
            };
        }

        let folder = '';
        const target = () => JSON.stringify([state.fileName, folder]);
        const sameTarget = () => !!state.savedPath && state.savedTarget === target();
        const contentChanged = () => !!state.savedPath && JSON.stringify(content()) !== state.savedRequest;
        const changedSinceSave = () => !!state.savedPath && (!sameTarget() || contentChanged());

        function render(busy = false) {
            const saved = !!state.savedPath;
            saveBtn.hidden = saved;
            openBtn.hidden = againBtn.hidden = !saved;
            for (const b of [saveBtn, openBtn, againBtn, newBtn]) b.disabled = busy;
            againBtn.classList.toggle('primary', changedSinceSave());
            againBtn.classList.toggle('secondary', !changedSinceSave());
            done.hidden = !saved || busy;
            if (!saved) return;
            const parts = state.savedPath.split(/[\\/]/);
            const name = parts.pop();
            done.replaceChildren(
                h('div.done-title', {}, '✔ 저장했습니다'),
                h('p', {}, h('b', {}, parts.join('\\')), ' 폴더에 ', h('b', {}, name), ' 파일로 저장했습니다.'),
                doneNote(),
            );
        }

        function doneNote() {
            if (!sameTarget()) {
                return h('p.done-changed', {}, '파일 이름이나 저장 위치가 바뀌었습니다. "다시 저장하기"를 누르면 새 파일로 저장합니다.');
            }
            if (contentChanged()) {
                return h('p.done-changed', {}, '저장한 뒤 바뀐 내용이 있습니다. "다시 저장하기"를 누르면 이 파일에 덮어쓸지 묻습니다.');
            }
            return h('p.done-hint', {}, '저장한 뒤 바뀐 내용이 없습니다. 이전 단계에서 고치거나 파일 이름을 바꾸면 다시 저장할 수 있습니다.');
        }

        async function saveAgain() {
            if (!changedSinceSave()) return;
            if (sameTarget() && contentChanged()) {
                const ok = await confirmDialog({
                    title: '같은 파일로 덮어쓰시겠습니까?',
                    lines: [`${state.savedPath.split(/[\\/]/).pop()} 파일을 바뀐 내용으로 바꿉니다.`, '덮어쓰면 이전 내용은 되돌릴 수 없습니다.'],
                    confirmText: '확인',
                    cancelText: '취소',
                });
                if (!ok) return;
            }
            save(sameTarget());
        }

        async function save(overwrite = false) {
            msg.clear();
            folderBtn.disabled = true;
            nameInput.disabled = true;
            ctx.setBusy(true);
            render(true);
            progress.start('export');
            off = api.onProgress(api.EVENTS.exportProgress, (p) => progress.update(p));
            const body = content();
            const dest = target();
            try {
                state.savedPath = await api.exportVideo({...body, fileName: state.fileName, overwrite});
                state.savedRequest = JSON.stringify(body);
                state.savedTarget = dest;
            } catch (e) {
                const text = api.errorText(e);
                msg.set(text, text === api.CANCELED_MESSAGE ? 'info' : 'error');
            } finally {
                off?.();
                off = null;
                progress.hide();
                folderBtn.disabled = false;
                nameInput.disabled = false;
                ctx.setBusy(false);
                render();
            }
        }

        async function startOver() {
            const saved = !!state.savedPath;
            const lines = !saved
                ? ['아직 이 영상을 저장하지 않았습니다.', '넘어가면 자른 구간과 가린 곳이 모두 사라지고 되돌릴 수 없습니다.']
                : changedSinceSave()
                    ? ['저장한 뒤 바꾼 내용은 아직 저장하지 않았습니다.', '넘어가면 바꾼 내용은 사라집니다. 이미 저장한 파일은 그대로 남아 있습니다.']
                    : ['저장한 영상 파일은 그대로 남아 있습니다.', '지금 편집하던 내용은 닫히고 처음 단계로 돌아갑니다.'];
            const ok = await confirmDialog({
                title: saved && !changedSinceSave() ? '새 영상 편집을 시작할까요?' : '저장하지 않은 내용이 있습니다',
                lines,
                confirmText: saved && !changedSinceSave() ? '새 영상 편집하기' : '저장 안 하고 넘어가기',
                cancelText: '계속 편집하기',
            });
            if (!ok) return;
            api.closeVideo();
            resetAll();
            ctx.goTo(0);
        }

        render();
        fillDefaults();
        return {unmount: () => off?.()};
    },
};
