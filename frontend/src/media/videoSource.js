// current는 단계 사이에 공유하는 video 요소 하나다.

import {h} from '../ui/dom.js';

let current = null;

// 같은 url이면 기존 요소를 그대로 쓰고, 다르면 이전 것을 닫고 새로 만든다.
export function openVideo(url) {
    if (current?.url === url) return current.video;
    closeVideo();
    const video = h('video.player-video', {
        src: url, preload: 'auto', playsInline: true, disablePictureInPicture: true, tabIndex: -1,
    });
    video.controls = false;
    current = {url, video};
    return video;
}

// src를 떼고 load()해 파일 연결을 놓는다.
export function closeVideo() {
    if (!current) return;
    const {video} = current;
    current = null;
    video.pause();
    video.remove();
    video.removeAttribute('src');
    video.load();
}
