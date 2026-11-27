// state는 단계 사이에 공유하는 편집 상태다. auto[kind] = {result, enabled, excluded}
// 타입은 Wails가 Go에서 만든 wailsjs/go/models.ts를 가리키고 QA의 tsc가 검사한다.
// rects·AutoBox 좌표는 레터박스를 뺀 영상 영역 기준 0~1 비율, 시각은 초다.
// segments는 2단계에서 넣은 구간(원본 초) 목록이고 순서가 곧 저장 순서다. 겹치거나 맞닿은 구간은
// 넣을 때 하나로 합쳐, 원본 시각 하나는 많아야 한 구간에만 있다. start·end는 2단계에서 고르는 중인 구간이다.
// 넣은 구간이 없으면 고르는 중인 구간 하나를 쓴다(clipSegments). 3·4단계는 구간을 순서대로 이어 붙인
// 시간("이어 붙인 시각", 0~totalLength)으로 보여 주고, outAt이 원본 시각을 그 시각으로 바꾼다.
// rects[i].from·to는 직접 가림을 켤 원본 시각이고, null이면 이어 붙인 영상의 처음·끝을 따른다.
// 켜는 시간은 이어 붙인 시각으로 from~to 사이라서(순서를 바꾼 구간에서도 보이는 대로), rectPieces가
// 구간마다 원본 조각으로 나눠 준다. 구간을 바꿔 from·to가 구간 밖이 되거나 순서가 뒤집히면
// 원본 시각 from~to와 겹치는 부분을 가린다(가림이 말없이 사라지지 않게).
// 따라간 가림은 따라간 원본 구간과 겹치는 부분만 켠다. 겹치거나 남는 시간이 없으면 조각이 없다.
// 그래서 구간을 다시 잘라도 "항상 가리기" 가림은 모든 구간을 가린다.
// rects[i].at은 그린 원본 시각, track은 "움직임 따라가기" 결과(Go가 준 추적 번호·프레임별 상자)다.
// track이 있으면 켜는 시간은 따라간 구간이고, 상자는 그 시각 프레임의 추적 상자를 TRACK_EXPAND만큼
// 넓힌 것이다(Go automask.followExpand와 같음, 저장 블러와 미리보기를 맞춤).
// savedPath는 마지막 저장 파일, savedRequest는 그때 내용(구간·가림·소리) JSON,
// savedTarget은 그때 [이름, 폴더] JSON이다. outputFolder는 Go가 알려 준 현재 저장 폴더다.

import {closeVideo} from './media/videoSource.js';

/** @typedef {import('../wailsjs/go/models').main.VideoInfo} VideoInfo */
/** @typedef {import('../wailsjs/go/models').main.AutoMask} AutoMask */
/** @typedef {import('../wailsjs/go/models').media.MaskRect} MaskRect */
/** @typedef {import('../wailsjs/go/models').main.TrackResult} TrackResult */

/**
 * @typedef {Object} ManualRect
 * @property {number} x
 * @property {number} y
 * @property {number} w
 * @property {number} h
 * @property {number | null} from
 * @property {number | null} to
 * @property {number} at
 * @property {TrackResult | null} track
 */

/**
 * @typedef {Object} Segment
 * @property {number} start
 * @property {number} end
 */

/**
 * @typedef {Object} AutoState
 * @property {AutoMask} result
 * @property {boolean} enabled
 * @property {number[]} excluded
 */

/**
 * @typedef {Object} EditState
 * @property {VideoInfo | null} video
 * @property {Segment[]} segments
 * @property {number} start
 * @property {number} end
 * @property {ManualRect[]} rects
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
    segments: [],
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

export const MAX_RECTS = 10;
export const MAX_SEGMENTS = 10;
export const MIN_CLIP_SECONDS = 1;

// 다른 url이면 이전 영상을 닫고, 구간은 전체(0.1초 내림)로 두고 나머지는 비운다.
/** @param {VideoInfo} video */
export function setVideo(video) {
    if (state.video?.url !== video.url) closeVideo();
    state.video = video;
    state.segments = [];
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
    state.segments = [];
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

// 고르는 중인 구간만 바꾼다. 자동 분석 결과는 버리지 않고, 빠진 부분은 3단계에서 이어서 분석한다.
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

// t가 든 분석 조각에서 (t - 조각 시작)·fps를 내림해 프레임을 고르고, 켜진 종류의 박스에 뺀 여부를 붙인다.
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
        for (const p of r.parts) {
            const n = Math.floor((t - p.start) * r.fps + 1e-3);
            if (n < 0 || n >= p.frames.length) continue;
            for (const [x, y, w, h, id] of p.frames[n] || []) {
                out.push({x, y, w, h, id, kind, excluded: a.excluded.includes(id)});
            }
            break;
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

// 이어 붙일 구간마다 그 구간을 덮는 분석 조각이 있는지 본다. 앞은 반 프레임, 끝은 1.5프레임 모자람까지 허용
/**
 * @param {string} kind
 * @returns {boolean}
 */
export function autoCovers(kind) {
    const r = state.auto[kind]?.result;
    if (!r) return false;
    const frame = 1 / r.fps;
    return clipSegments().every((s) => r.parts.some((p) =>
        p.start <= s.start + frame / 2 && p.start + p.frames.length * frame >= s.end - frame * 1.5));
}

// 켜진 종류만 {kind: [ID...]}로 복사한다.
/** @returns {Record<string, number[]>} */
export function autoExclusions() {
    return Object.fromEntries(enabledAuto().map((k) => [k, [...state.auto[k].excluded]]));
}

// 영상 유무·순서·최소 길이를 보고, ok가 false면 message에 안내
/**
 * @param {number} [start]
 * @param {number} [end]
 * @returns {{ok: boolean, message: string}}
 */
export function checkRange(start = state.start, end = state.end) {
    if (!state.video) return {ok: false, message: '먼저 영상을 불러와 주세요.'};
    if (end <= start) {
        return {ok: false, message: '끝 지점이 시작 지점보다 앞에 있습니다. "여기서 끝"을 다시 눌러 주세요.'};
    }
    if (end - start < MIN_CLIP_SECONDS) {
        return {ok: false, message: '자를 구간이 너무 짧습니다. 시작과 끝을 1초 이상 떨어지게 정해 주세요.'};
    }
    return {ok: true, message: ''};
}

// 넣은 구간이 있으면 그 목록, 없으면 고르는 중인 구간이 맞을 때 그 하나, 아니면 빈 목록
/** @returns {Segment[]} */
export function clipSegments() {
    if (state.segments.length) return state.segments;
    return checkRange().ok ? [{start: state.start, end: state.end}] : [];
}

// 이어 붙인 전체 길이(초)
export function totalLength() {
    return clipSegments().reduce((sum, s) => sum + s.end - s.start, 0);
}

// 원본 시각 t가 든 구간 번호(없으면 -1). 끝 시각은 다음 구간 쪽으로 본다.
/** @param {number} t */
export function segmentIndexAt(t) {
    return clipSegments().findIndex((s) => t >= s.start - 1e-3 && t < s.end - 1e-3);
}

// 원본 시각 → 이어 붙인 시각. 어느 구간에도 없으면 null
/**
 * @param {number} t
 * @returns {number | null}
 */
export function outAt(t) {
    let offset = 0;
    for (const s of clipSegments()) {
        if (t >= s.start - 1e-3 && t <= s.end + 1e-3) return offset + Math.min(Math.max(t, s.start), s.end) - s.start;
        offset += s.end - s.start;
    }
    return null;
}

// 구간마다 [이어 붙인 시작, 이어 붙인 끝]
/** @returns {[number, number][]} */
export function segmentOuts() {
    let offset = 0;
    return clipSegments().map((s) => {
        const out = /** @type {[number, number]} */ ([offset, offset + s.end - s.start]);
        offset = out[1];
        return out;
    });
}

// 겹치거나 맞닿은 구간과 합쳐 넣고(replace면 그 자리) 넣은 번호·합친 수를 준다
/**
 * @param {Segment} seg
 * @param {number} [replace]
 * @returns {{index: number, merged: number}}
 */
export function putSegment(seg, replace = -1) {
    const list = state.segments.map((s, i) => ({...s, i}));
    if (replace >= 0) list.splice(replace, 1);
    const hits = list.filter((s) => s.end >= seg.start && s.start <= seg.end);
    const joined = hits.reduce((a, s) => ({start: Math.min(a.start, s.start), end: Math.max(a.end, s.end)}), seg);
    const firstHit = hits.length ? hits[0].i : Infinity;
    const at = Math.min(replace >= 0 ? replace : Infinity, firstHit);
    const rest = list.filter((s) => !hits.includes(s));
    const pos = Number.isFinite(at) ? rest.filter((s) => s.i < at).length : rest.length;
    rest.splice(pos, 0, {...joined, i: -1});
    state.segments = rest.map(({start, end}) => ({start, end}));
    return {index: pos, merged: hits.length};
}

// i번 구간을 d(−1 위로, +1 아래로)만큼 옮긴다. 옮겼으면 true
/**
 * @param {number} i
 * @param {number} d
 */
export function moveSegment(i, d) {
    const j = i + d;
    if (j < 0 || j >= state.segments.length) return false;
    [state.segments[i], state.segments[j]] = [state.segments[j], state.segments[i]];
    return true;
}

const TRACK_EXPAND = 0.2;

// 원본 [a, b]와 구간마다 겹치는 부분(저장 순서)
/**
 * @param {number} a
 * @param {number} b
 * @returns {[number, number][]}
 */
function sourcePieces(a, b) {
    return clipSegments().flatMap((s) => (Math.min(b, s.end) > Math.max(a, s.start) ? [[Math.max(a, s.start), Math.min(b, s.end)]] : []));
}

// 따라간 구간, 또는 이어 붙인 from~to(안 되면 원본 from~to)를 구간마다 원본 조각으로 나눈다
/**
 * @param {ManualRect} r
 * @returns {[number, number][]}
 */
export function rectPieces(r) {
    if (r.track) return sourcePieces(r.track.start, r.track.start + r.track.frames.length / r.track.fps);
    const segs = clipSegments();
    const from = r.from === null ? 0 : outAt(r.from);
    const to = r.to === null ? totalLength() : outAt(r.to);
    /** @type {[number, number][]} */
    const joined = from === null || to === null || to <= from ? [] : segmentOuts().flatMap(([o0, o1], i) => {
        const a = Math.max(o0, from);
        const b = Math.min(o1, to);
        return b > a ? [[segs[i].start + a - o0, segs[i].start + b - o0]] : [];
    });
    return joined.length ? joined : sourcePieces(r.from ?? -Infinity, r.to ?? Infinity);
}

// 추적한 가림이면 t 프레임(내림) 상자를 넓혀 0~1로 자르고, 아니면 그린 상자 그대로
/**
 * @param {ManualRect} r
 * @param {number} t
 * @returns {{x: number, y: number, w: number, h: number}}
 */
export function rectBoxAt(r, t) {
    const tr = r.track;
    if (!tr) return {x: r.x, y: r.y, w: r.w, h: r.h};
    const n = Math.min(tr.frames.length - 1, Math.max(0, Math.floor((t - tr.start) * tr.fps + 1e-3)));
    const [x, y, w, h] = tr.frames[n];
    const dx = w * TRACK_EXPAND / 2;
    const dy = h * TRACK_EXPAND / 2;
    const x0 = Math.max(0, x - dx);
    const y0 = Math.max(0, y - dy);
    return {x: x0, y: y0, w: Math.min(1, x + w + dx) - x0, h: Math.min(1, y + h + dy) - y0};
}

// rectPieces 안이면 true. 끝 시각 프레임은 빼려고 1ms 앞당겨 비교한다.
/**
 * @param {ManualRect} r
 * @param {number} t
 */
export function rectActiveAt(r, t) {
    return rectPieces(r).some(([a, b]) => t >= a - 1e-3 && t < b - 1e-3);
}

// 사각형마다 켜지는 원본 조각을 저장 요청 형태로 바꾼다(조각마다 하나씩, Go가 구간별로 다시 자름).
/** @returns {MaskRect[]} */
export function rectsForExport() {
    return state.rects.flatMap((r) => rectPieces(r).map(([start, end]) =>
        ({x: r.x, y: r.y, w: r.w, h: r.h, start, end, track: r.track?.id ?? 0})));
}
