import {closeVideo} from './media/videoSource.js';

// 단계 사이에 공유하는 편집 상태
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

// 새 영상: 이전 영상을 닫고 구간·가림 초기화
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

// 모든 상태를 비우고 영상을 닫는다.
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

// 구간이 바뀌면 자동 가림 결과를 버린다.
export function setRange(start, end) {
    if (start !== state.start || end !== state.end) state.auto = {};
    state.start = start;
    state.end = end;
}

// 켜진 자동 가림 종류 목록
export function enabledAuto() {
    return Object.keys(state.auto).filter((k) => state.auto[k].enabled);
}

// 시각 t 화면 프레임(내림)의 자동 박스와 뺀 여부
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

// 자동 트랙 하나를 빼거나 되살린다. 뺐으면 true
export function toggleExcluded(kind, id) {
    const a = state.auto[kind];
    const i = a.excluded.indexOf(id);
    if (i >= 0) a.excluded.splice(i, 1);
    else a.excluded.push(id);
    return i < 0;
}

// 저장 요청용: 종류별로 뺀 트랙 번호
export function autoExclusions() {
    return Object.fromEntries(enabledAuto().map((k) => [k, [...state.auto[k].excluded]]));
}

// 구간 검사. ok가 false면 message에 안내
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
