// 화면 코드는 이 파일로만 Go와 통신한다.

import * as App from '../wailsjs/go/main/App.js';
import {EventsOn} from '../wailsjs/runtime/runtime.js';

export const download = (url) => App.Download(url);
export const openLocal = () => App.OpenLocal();
export const closeVideo = () => App.CloseVideo();
export const analyze = (req) => App.Analyze(req);
export const exportVideo = (req) => App.Export(req);
export const cancel = () => App.Cancel();
export const outputFolder = () => App.OutputFolder();
export const defaultFileName = () => App.DefaultFileName();
export const chooseOutputFolder = () => App.ChooseOutputFolder();
export const openOutputFolder = () => App.OpenOutputFolder();

// `${kind}:progress` 이벤트를 구독하고 해제 함수를 돌려준다.
export function onProgress(kind, cb) {
    return EventsOn(`${kind}:progress`, cb);
}

export const CANCELED_MESSAGE = '작업을 취소했습니다.';

// 문자열·Error·그 밖의 값에서 문장을 꺼내고, 비었으면 기본 안내문
export function errorText(err) {
    const s = typeof err === 'string' ? err : err?.message || String(err ?? '');
    return s || '알 수 없는 문제가 생겼습니다. 프로그램을 다시 켜 주세요.';
}
