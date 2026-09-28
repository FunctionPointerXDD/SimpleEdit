import {h} from '../ui/dom.js';

// 단계 사이에 공유하는 video 요소 하나
let current = null;

// url의 video를 돌려준다. 다른 url이면 새로 연다.
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

// 영상을 닫고 파일 연결을 놓는다.
export function closeVideo() {
    if (!current) return;
    const {video} = current;
    current = null;
    video.pause();
    video.remove();
    video.removeAttribute('src');
    video.load();
}
