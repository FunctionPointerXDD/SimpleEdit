// state는 단계 사이에 공유하는 편집 상태다. auto[kind] = {result, enabled, excluded}
// 타입은 Wails가 Go에서 만든 wailsjs/go/models.ts를 가리키고 QA의 tsc가 검사한다.
// rects·AutoBox 좌표는 레터박스를 뺀 영상 영역 기준 0~1 비율, 시각은 초다.
// savedPath는 마지막 저장 파일, savedRequest는 그때 내용(구간·가림·소리) JSON,
// savedTarget은 그때 [이름, 폴더] JSON이다. outputFolder는 Go가 알려 준 현재 저장 폴더다.

import {closeVideo} from './media/videoSource.js';

/** @typedef {import('../wailsjs/go/models').main.VideoInfo} VideoInfo */
/** @typedef {import('../wailsjs/go/models').main.AutoMask} AutoMask */
/** @typedef {import('../wailsjs/go/models').media.Rect} Rect */

/**
 * @typedef {Object} AutoState
 * @property {AutoMask} result
 * @property {boolean} enabled
 * @property {number[]} excluded
 */

/**
 * @typedef {Object} EditState
 * @property {VideoInfo | null} video
 * @property {number} start
 * @property {number} end
 * @property {Rect[]} rects
 * @property {Record<string, AutoState>} auto
 * @property {boolean} mute
 * @property {string} fileName
 * @property {string} savedPath
 * @property {string} savedRequest
 * @property {string} savedTarget
 * @property {string} outputFolder
 */

/**
 * @typedef {Object} AutoBox
 * @property {number} x
 * @property {number} y
 * @property {number} w
 * @property {number} h
 * @property {number} id
 * @property {string} kind
 * @property {boolean} excluded
 */

/** @type {EditState} */
export const state = {
    video: null,
    start: 0,
    end: 0,
    rects: [],
    auto: {},
    mute: false,
    fileName: '',
    savedPath: '',
    savedRequest: '',
    savedTarget: '',
    outputFolder: '',
};

export const MAX_RECTS = 5;
export const MIN_CLIP_SECONDS = 1;

// 다른 url이면 이전 영상을 닫고, 구간은 전체(0.1초 내림)로 두고 나머지는 비운다.
/** @param {VideoInfo} video */
export function setVideo(video) {
    if (state.video?.url !== video.url) closeVideo();
    state.video = video;
    state.start = 0;
    state.end = Math.floor(video.duration * 10) / 10;
    state.rects = [];
    state.auto = {};
    state.mute = false;
    state.fileName = '';
    state.savedPath = '';
    state.savedRequest = '';
    state.savedTarget = '';
}

export function resetAll() {
    closeVideo();
    state.video = null;
    state.start = 0;
    state.end = 0;
    state.rects = [];
    state.auto = {};
    state.mute = false;
    state.fileName = '';
    state.savedPath = '';
    state.savedRequest = '';
    state.savedTarget = '';
}

// 자동 분석 결과는 버리지 않는다. 빠진 부분은 3단계에서 이어서 분석한다.
/**
 * @param {number} start
 * @param {number} end
 */
export function setRange(start, end) {
    state.start = start;
    state.end = end;
}

/** @returns {string[]} */
export function enabledAuto() {
    return Object.keys(state.auto).filter((k) => state.auto[k].enabled);
}

// (t - start)·fps를 내림해 프레임을 고르고, 켜진 종류의 박스에 뺀 여부를 붙인다.
/**
 * @param {number} t
 * @returns {AutoBox[]}
 */
export function autoRectsAt(t) {
    /** @type {AutoBox[]} */
    const out = [];
    for (const kind of enabledAuto()) {
        const a = state.auto[kind];
        const r = a.result;
        const n = Math.floor((t - r.start) * r.fps + 1e-3);
        if (n < 0 || n >= r.frames.length) continue;
        for (const [x, y, w, h, id] of r.frames[n] || []) {
            out.push({x, y, w, h, id, kind, excluded: a.excluded.includes(id)});
        }
    }
    return out;
}

// excluded에 있으면 빼고 없으면 넣는다. 뺐으면 true
/**
 * @param {string} kind
 * @param {number} id
 * @returns {boolean}
 */
export function toggleExcluded(kind, id) {
    const a = state.auto[kind];
    const i = a.excluded.indexOf(id);
    if (i >= 0) a.excluded.splice(i, 1);
    else a.excluded.push(id);
    return i < 0;
}

// 분석 범위가 [start, end]를 덮는지 본다. 앞은 반 프레임, 끝은 1.5프레임 모자람까지 허용
/**
 * @param {string} kind
 * @returns {boolean}
 */
export function autoCovers(kind) {
    const r = state.auto[kind]?.result;
    if (!r) return false;
    const frame = 1 / r.fps;
    return r.start <= state.start + frame / 2 && r.start + r.frames.length * frame >= state.end - frame * 1.5;
}

// 켜진 종류만 {kind: [ID...]}로 복사한다.
/** @returns {Record<string, number[]>} */
export function autoExclusions() {
    return Object.fromEntries(enabledAuto().map((k) => [k, [...state.auto[k].excluded]]));
}

// 영상 유무·순서·최소 길이를 보고, ok가 false면 message에 안내
/** @returns {{ok: boolean, message: string}} */
export function checkRange() {
    if (!state.video) return {ok: false, message: '먼저 영상을 불러와 주세요.'};
    if (state.end <= state.start) {
        return {ok: false, message: '끝 지점이 시작 지점보다 앞에 있습니다. "여기서 끝"을 다시 눌러 주세요.'};
    }
    if (state.end - state.start < MIN_CLIP_SECONDS) {
        return {ok: false, message: '자를 구간이 너무 짧습니다. 시작과 끝을 1초 이상 떨어지게 정해 주세요.'};
    }
    return {ok: true, message: ''};
}
