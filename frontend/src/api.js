// 화면 코드는 이 파일로만 Go와 통신한다.
// 요청·응답 타입은 Wails가 Go에서 만든 wailsjs/go/models.ts를 JSDoc으로 가리켜 QA의 tsc가 검사한다.
// 요청은 필드만 맞으면 되므로 생성된 클래스에서 convertValues 메서드를 뺀 모양(Plain)을 받는다.
// 명세 전체는 docs/api.json, 이벤트 이름·안내 문장은 apiSpec.js(둘 다 cmd/apispec이 만듦)에 있다.

import * as App from '../wailsjs/go/main/App.js';
import {EventsOn} from '../wailsjs/runtime/runtime.js';
import {EVENTS, MESSAGES} from './apiSpec.js';

export {EVENTS};

/** @typedef {import('../wailsjs/go/models').main.AnalyzeRequest} AnalyzeRequest */
/** @typedef {import('../wailsjs/go/models').main.ExportRequest} ExportRequest */
/** @typedef {import('./apiSpec.js').ProgressEvent} ProgressEvent */
/** @template T @typedef {Omit<T, 'convertValues'>} Plain */

/** @param {string} url */
export const download = (url) => App.Download(url);
export const openLocal = () => App.OpenLocal();
export const closeVideo = () => App.CloseVideo();
/** @param {Plain<AnalyzeRequest>} req */
export const analyze = (req) => App.Analyze(/** @type {AnalyzeRequest} */ (req));
/** @param {Plain<ExportRequest>} req */
export const exportVideo = (req) => App.Export(/** @type {ExportRequest} */ (req));
export const cancel = () => App.Cancel();
export const outputFolder = () => App.OutputFolder();
export const defaultFileName = () => App.DefaultFileName();
export const chooseOutputFolder = () => App.ChooseOutputFolder();
export const openOutputFolder = () => App.OpenOutputFolder();

// EVENTS의 이벤트를 구독하고 해제 함수를 돌려준다.
/**
 * @param {typeof EVENTS[keyof typeof EVENTS]} event
 * @param {(p: ProgressEvent) => void} cb
 */
export function onProgress(event, cb) {
    return EventsOn(event, cb);
}

export const CANCELED_MESSAGE = MESSAGES.msgCanceled;

// 문자열·Error·그 밖의 값에서 문장을 꺼내고, 비었으면 기본 안내문
export function errorText(err) {
    const s = typeof err === 'string' ? err : err?.message || String(err ?? '');
    return s || '알 수 없는 문제가 생겼습니다. 프로그램을 다시 켜 주세요.';
}
