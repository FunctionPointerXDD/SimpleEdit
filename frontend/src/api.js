import * as App from '../wailsjs/go/main/App.js';
import {EventsOn} from '../wailsjs/runtime/runtime.js';

// 화면 코드는 이 파일로만 Go와 통신한다.
export const download = (url) => App.Download(url);
export const openLocal = () => App.OpenLocal();
export const closeVideo = () => App.CloseVideo();
export const exportVideo = (req) => App.Export(req);
export const cancel = () => App.Cancel();
export const outputFolder = () => App.OutputFolder();
export const defaultFileName = () => App.DefaultFileName();
export const chooseOutputFolder = () => App.ChooseOutputFolder();
export const openOutputFolder = () => App.OpenOutputFolder();

// kind: 'download' | 'export'. 반환 함수로 해제
export function onProgress(kind, cb) {
    return EventsOn(`${kind}:progress`, cb);
}

export const CANCELED_MESSAGE = '작업을 취소했습니다.';

// Go 오류를 화면에 보일 문장으로 바꾼다.
export function errorText(err) {
    const s = typeof err === 'string' ? err : err?.message || String(err ?? '');
    return s || '알 수 없는 문제가 생겼습니다. 프로그램을 다시 켜 주세요.';
}
