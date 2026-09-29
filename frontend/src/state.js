// state는 단계 사이에 공유하는 편집 상태다. auto[kind] = {result, enabled, excluded}

import {closeVideo} from './media/videoSource.js';

export const state = {
    video: null,
    start: 0,
    end: 0,
    rects: [],
    auto: {},
    mute: false,
    fileName: '',
    savedPath: '',
};

export const MAX_RECTS = 5;
export const MIN_CLIP_SECONDS = 1;

// 다른 url이면 이전 영상을 닫고, 구간은 전체(0.1초 내림)로 두고 나머지는 비운다.
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
}

// 값이 달라질 때만 state.auto를 비운다.
export function setRange(start, end) {
    if (start !== state.start || end !== state.end) state.auto = {};
    state.start = start;
    state.end = end;
}

export function enabledAuto() {
    return Object.keys(state.auto).filter((k) => state.auto[k].enabled);
}

// (t - start)·fps를 내림해 프레임을 고르고, 켜진 종류의 박스에 뺀 여부를 붙인다.
export function autoRectsAt(t) {
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
export function toggleExcluded(kind, id) {
    const a = state.auto[kind];
    const i = a.excluded.indexOf(id);
    if (i >= 0) a.excluded.splice(i, 1);
    else a.excluded.push(id);
    return i < 0;
}

// 켜진 종류만 {kind: [ID...]}로 복사한다.
export function autoExclusions() {
    return Object.fromEntries(enabledAuto().map((k) => [k, [...state.auto[k].excluded]]));
}

// 영상 유무·순서·최소 길이를 보고, ok가 false면 message에 안내
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
