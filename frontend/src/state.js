import {closeVideo} from './media/videoSource.js';

// 단계 사이에 공유하는 편집 상태
export const state = {
    video: null,
    start: 0,
    end: 0,
    rects: [],
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
    state.fileName = '';
    state.savedPath = '';
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
