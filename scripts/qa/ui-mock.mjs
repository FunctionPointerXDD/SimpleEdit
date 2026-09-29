// 모의 백엔드(window.go, window.runtime)로 마법사 전체를 조작하는 UI 테스트
// 사용: node ui-mock.mjs <wide|tall>  (run.sh가 영상 생성과 실행을 맡음)
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import {chromium} from 'playwright-core';

const HERE = path.dirname(new URL(import.meta.url).pathname);
const DIST = path.join(HERE, '..', '..', 'frontend', 'dist');
const CACHE = path.join(HERE, '.cache');
const VIDEO = process.argv[2] || 'wide';
const OUT = path.join(CACHE, 'shots', VIDEO);
fs.mkdirSync(OUT, {recursive: true});

const TYPES = {'.js': 'text/javascript', '.css': 'text/css', '.webm': 'video/webm', '.html': 'text/html'};

function serve(req, res) {
    const p = decodeURIComponent(new URL(req.url, 'http://x').pathname);
    const file = p.startsWith('/video/') ? path.join(CACHE, p.slice(7)) : path.join(DIST, p === '/' ? 'index.html' : p);
    if (!fs.existsSync(file)) {
        res.writeHead(404).end();
        return;
    }
    const size = fs.statSync(file).size;
    const type = TYPES[path.extname(file)] || 'application/octet-stream';
    const m = /bytes=(\d*)-(\d*)/.exec(req.headers.range || '');
    if (m) {
        const start = m[1] ? +m[1] : 0;
        const end = m[2] ? +m[2] : size - 1;
        res.writeHead(206, {'Content-Type': type, 'Content-Range': `bytes ${start}-${end}/${size}`,
            'Content-Length': end - start + 1, 'Accept-Ranges': 'bytes'});
        fs.createReadStream(file, {start, end}).pipe(res);
    } else {
        res.writeHead(200, {'Content-Type': type, 'Content-Length': size, 'Accept-Ranges': 'bytes'});
        fs.createReadStream(file).pipe(res);
    }
}

// 브라우저에 넣는 가짜 Go 바인딩
function installMock({video, meta}) {
    const handlers = {};
    const emit = (name, data) => (handlers[name] || []).forEach((cb) => cb(data));
    window.runtime = {
        EventsOnMultiple(name, cb) {
            (handlers[name] ||= []).push(cb);
            return () => { handlers[name] = handlers[name].filter((x) => x !== cb); };
        },
        EventsOff() {},
    };
    const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
    async function fakeProgress(name, stages, ms) {
        for (const stage of stages) {
            for (let p = 0; p <= 100; p += 20) {
                emit(name, {percent: p, remaining: (100 - p) / 10, stage});
                await sleep(ms);
            }
        }
    }
    window.__mock = {exportReq: null, analyzeReq: null, failDownload: null, closed: 0,
        folder: 'C:\\Users\\강사\\Desktop'};
    window.go = {main: {App: {
        Download: async () => {
            if (window.__mock.failDownload) {
                await sleep(200);
                throw window.__mock.failDownload;
            }
            await fakeProgress('download:progress', ['start', 'download', 'merge'], 60);
            return {url: `/video/${video}.webm`, duration: 20, width: meta.w, height: meta.h, name: '유튜브 영상'};
        },
        OpenLocal: async () => ({}),
        CloseVideo: async () => { window.__mock.closed++; },
        Analyze: async (req) => {
            window.__mock.analyzeReq = req;
            await fakeProgress('analyze:progress', ['analyze'], 40);
            const fps = 30;
            const n = Math.round((req.end - req.start) * fps);
            const frames = Array.from({length: n}, (_, i) => [[0.35 + 0.1 * i / n, 0.2, 0.2, 0.3, 1]]);
            return {kind: req.kind, start: req.start, duration: req.end - req.start, fps, frames, found: n};
        },
        Export: async (req) => {
            window.__mock.exportReq = req;
            await fakeProgress('export:progress', ['export'], 80);
            return 'C:\\Users\\강사\\Desktop\\운전강의_20260928_1405.mp4';
        },
        Cancel: async () => {},
        OutputFolder: async () => window.__mock.folder,
        DefaultFileName: async () => '운전강의_20260928_1405',
        ChooseOutputFolder: async () => { window.__mock.folder = 'D:\\강의영상'; return window.__mock.folder; },
        OpenOutputFolder: async () => {},
    }}};
}

const server = http.createServer(serve);
await new Promise((r) => server.listen(0, r));
const base = `http://127.0.0.1:${server.address().port}`;
const meta = VIDEO === 'wide' ? {w: 1280, h: 720} : {w: 540, h: 960};

const browser = await chromium.launch({executablePath: process.env.CHROME});
const page = await browser.newPage({viewport: {width: 1280, height: 770}});
const logs = [];
page.on('console', (m) => logs.push(`[${m.type()}] ${m.text()}`));
page.on('pageerror', (e) => logs.push(`[pageerror] ${e.message}`));
await page.addInitScript(installMock, {video: VIDEO, meta});

const results = [];
const check = (name, ok, detail = '') => results.push(`${ok ? 'PASS' : 'FAIL'} ${name} ${detail}`);
const shot = (n) => page.screenshot({path: path.join(OUT, `${n}.png`)});
const click = (text) => page.getByRole('button', {name: text, exact: true}).click();
const nextBtn = () => page.getByRole('button', {name: '다음 ▶'});
const videoReady = () => page.waitForFunction(() => document.querySelector('video')?.readyState >= 1);
const seekTo = (t) => page.$eval('input.seek', (el, v) => {
    el.value = String(v);
    el.dispatchEvent(new Event('input'));
    el.dispatchEvent(new Event('change'));
}, t);
const orangeCount = () => page.$eval('.mask-canvas', (c) => {
    const d = c.getContext('2d').getImageData(0, 0, c.width, c.height).data;
    let n = 0;
    for (let i = 0; i < d.length; i += 4) if (d[i] > 240 && d[i + 1] > 130 && d[i + 1] < 170 && d[i + 2] < 30) n++;
    return n;
});

// 1단계
await page.goto(base);
await shot('1-start');
check('상단 단계 표시', (await page.textContent('.step-label')).includes('1 / 4'));
check('첫 화면 이전 버튼 숨김', await page.$eval('.navbar .btn.nav', (b) => getComputedStyle(b).visibility === 'hidden'));
check('다음 버튼 비활성', await nextBtn().isDisabled());
check('붙여넣기 버튼 없음', (await page.getByRole('button', {name: '붙여넣기'}).count()) === 0);
check('주소 칸에 포커스', await page.evaluate(() => document.activeElement?.classList.contains('url-input')));
await click('영상 받기');
check('빈 주소 안내', (await page.textContent('.message.error')).includes('비어'));
await page.evaluate(() => { window.__mock.failDownload = '영상을 받지 못했습니다. 인터넷 연결을 확인하고 다시 눌러 주세요.'; });
await page.fill('.url-input', 'https://youtu.be/dQw4w9WgXcQ?si=abc');
await click('영상 받기');
await page.waitForSelector('.message.error');
check('다운로드 실패 안내', (await page.textContent('.message.error')).includes('인터넷'));
await page.evaluate(() => { window.__mock.failDownload = null; });
await page.press('.url-input', 'Enter');
await page.waitForSelector('.progress:not([hidden])');
check('다운로드 중 다음 잠금', await nextBtn().isDisabled());
await page.waitForSelector('.loaded-card:not([hidden])');
await shot('1-loaded');

// 2단계
await click('다음 ▶');
await videoReady();
await page.evaluate(() => { document.querySelector('video').__mark = 'same'; });
let videoRequests = 0;
page.on('request', (r) => { if (r.url().includes('/video/')) videoRequests++; });
await seekTo(8);
await page.waitForTimeout(300);
await click('여기서 끝');
await seekTo(12);
await page.waitForTimeout(300);
await click('여기서 시작');
check('끝보다 늦은 시작 → 오류 안내', (await page.$('.guide-line.error')) !== null);
check('오류 시 다음 비활성', await nextBtn().isDisabled());
await seekTo(3);
await page.waitForTimeout(300);
await click('여기서 시작');
await seekTo(13.5);
await page.waitForTimeout(300);
await click('여기서 끝');
await page.waitForTimeout(200);
await shot('2-trim');
const vals = await page.$$eval('.mark-card .value', (els) => els.map((e) => e.textContent));
check('시작/끝/길이 표시', vals[0] === '00:03.0' && vals[1] === '00:13.5' && vals[2] === '10.5초', JSON.stringify(vals));

// 3단계: 직접 가릴 곳
await click('다음 ▶');
await videoReady();
check('3단계도 같은 video 요소', await page.evaluate(() => document.querySelector('video').__mark === 'same'));
check('3단계 시작 위치 = 시작 지점', await page.evaluate(() => Math.abs(document.querySelector('video').currentTime - 3) < 0.05));
await page.waitForTimeout(300);
const box = await page.$eval('video', (v) => {
    const r = v.getBoundingClientRect();
    const s = Math.min(r.width / v.videoWidth, r.height / v.videoHeight);
    const w = v.videoWidth * s;
    const h = v.videoHeight * s;
    return {x: r.left + (r.width - w) / 2, y: r.top + (r.height - h) / 2, w, h};
});
async function drag(fx0, fy0, fx1, fy1) {
    await page.mouse.move(box.x + box.w * fx0, box.y + box.h * fy0);
    await page.mouse.down();
    await page.mouse.move(box.x + box.w * (fx0 + fx1) / 2, box.y + box.h * (fy0 + fy1) / 2, {steps: 4});
    await page.mouse.move(box.x + box.w * fx1, box.y + box.h * fy1, {steps: 4});
    await page.mouse.up();
}
await drag(0.1, 0.2, 0.4, 0.5);
await drag(0.9, 0.9, 0.6, 0.7);
await drag(0.5, 0.5, 0.505, 0.505);
check('사각형 2개 목록', (await page.$$('.rect-list li:not(.empty)')).length === 2);
for (let i = 0; i < 4; i++) await drag(0.02 + i * 0.12, 0.02, 0.12 + i * 0.12, 0.12);
check('최대 5개 제한 안내', (await page.textContent('.guide-line')).includes('5개'));
for (let i = 0; i < 3; i++) await page.getByRole('button', {name: '지우기'}).last().click();
check('지우기 후 2개', (await page.$$('.rect-list li:not(.empty)')).length === 2);

// 3단계: 자동 가리기
check('자동 전에는 주황 박스 없음', (await orangeCount()) === 0);
await click('사람 얼굴 가리기');
await page.waitForSelector('.auto-panel .progress:not([hidden])');
check('분석 중 다음 잠금', await nextBtn().isDisabled());
await page.waitForSelector('.auto-toggle:not([hidden])');
await page.waitForTimeout(200);
check('분석 요청 구간', await page.evaluate(() => window.__mock.analyzeReq.start === 3 && window.__mock.analyzeReq.end === 13.5));
check('주황 박스 표시', (await orangeCount()) > 100);
await shot('3-auto-done');
await click('얼굴 가리기 끄기');
await page.waitForTimeout(100);
check('끄면 주황 박스 사라짐', (await orangeCount()) === 0);
await click('얼굴 가리기 켜기');

// 자동 박스 하나 빼기: 끌지 않고 누르기
const clickAt = async (fx, fy) => {
    await page.mouse.move(box.x + box.w * fx, box.y + box.h * fy);
    await page.mouse.down();
    await page.mouse.up();
    await page.waitForTimeout(100);
};
await clickAt(0.45, 0.35);
check('주황 박스 누르면 빠짐', (await orangeCount()) === 0 && (await page.textContent('.guide-line')).includes('뺐습니다'));
check('빼기는 직접 가릴 곳을 늘리지 않음', (await page.$$('.rect-list li:not(.empty)')).length === 2);
await shot('3-excluded');
await clickAt(0.45, 0.35);
check('다시 누르면 되살아남', (await orangeCount()) > 100);
await clickAt(0.45, 0.35);

// 3단계: 소리 없애기
check('처음엔 소리 켜짐', await page.evaluate(() => document.querySelector('video').muted === false));
await click('소리 없애기');
check('소리 없애기 → 미리보기 음소거', await page.evaluate(() => document.querySelector('video').muted === true));
check('버튼이 되살리기로 바뀜', (await page.getByRole('button', {name: '소리 되살리기'}).count()) === 1);
await shot('3-mute');

// 4단계
await click('다음 ▶');
check('4단계에서는 음소거 해제', await page.evaluate(() => !document.querySelector('video') || document.querySelector('video').muted === false));
check('4단계 다음 버튼 숨김', await nextBtn().isHidden());
const summary = await page.textContent('.summary');
check('요약에 자동 가림', summary.includes('사람 얼굴'));
check('요약에 소리 없앰', summary.includes('없앰'));
check('기본 파일 이름', (await page.inputValue('.name-input')) === '운전강의_20260928_1405');
await page.fill('.name-input', '교차로 사고 영상');
await click('저장 위치 바꾸기');
await page.waitForFunction(() => document.querySelector('.folder-path')?.textContent.includes('강의영상'));
check('저장 위치 바꾸기 표시', true);
await click('저장하기');
await page.waitForTimeout(250);
check('저장 중 이전 잠금', await page.getByRole('button', {name: '◀ 이전'}).isDisabled());
await page.waitForSelector('.done-card:not([hidden])');
await shot('4-done');
const req = await page.evaluate(() => window.__mock.exportReq);
const near = (a, b) => Math.abs(a - b) < 0.01;
check('저장 요청 파일 이름', req.fileName === '교차로 사고 영상', req.fileName);
check('저장 요청 자동 가림', JSON.stringify(req.auto) === '["face"]', JSON.stringify(req.auto));
check('저장 요청 무음', req.mute === true, String(req.mute));
check('저장 요청 뺀 트랙', JSON.stringify(req.exclude) === '{"face":[1]}', JSON.stringify(req.exclude));
check('저장 요청 구간', req.start === 3 && req.end === 13.5);
check('사각형 1 비율', near(req.rects[0].x, 0.1) && near(req.rects[0].y, 0.2) && near(req.rects[0].w, 0.3) && near(req.rects[0].h, 0.3), JSON.stringify(req.rects[0]));
check('사각형 2 비율(역방향 드래그)', near(req.rects[1].x, 0.6) && near(req.rects[1].y, 0.7) && near(req.rects[1].w, 0.3) && near(req.rects[1].h, 0.2), JSON.stringify(req.rects[1]));
check('사각형 개수', req.rects.length === 2);
check('완료 안내에 폴더', (await page.textContent('.done-card')).includes('Desktop'));

// 저장 후 이전 → 영상·가림·무음 유지
await click('◀ 이전');
await videoReady();
check('저장 후 이전: 같은 video', await page.evaluate(() => document.querySelector('video').__mark === 'same'));
check('저장 후 이전: 가림 유지', (await page.$$('.rect-list li:not(.empty)')).length === 2);
check('저장 후 이전: 무음 유지', await page.evaluate(() => document.querySelector('video').muted === true));
check('영상 파일 재요청 없음', videoRequests === 0, `요청 ${videoRequests}회`);
check('CloseVideo 아직 안 부름', await page.evaluate(() => window.__mock.closed === 0));

// 구간을 바꾸면 자동 결과 버림
await click('◀ 이전');
await videoReady();
await seekTo(4);
await page.waitForTimeout(200);
await click('여기서 시작');
await click('다음 ▶');
await page.waitForTimeout(300);
check('구간 바꾸면 자동 결과 버림', (await page.$('.auto-toggle[hidden]')) !== null);
await click('다음 ▶');
await page.waitForFunction(() => document.querySelector('.name-input')?.value !== '');
check('이전 후 돌아와도 파일 이름 유지', (await page.inputValue('.name-input')) === '교차로 사고 영상');
await page.focus('.name-input');
await page.keyboard.press('Enter');
await page.waitForSelector('.done-card:not([hidden])');
await page.evaluate(() => { window.__mock.exportReq = null; });
await page.focus('.name-input');
await page.keyboard.press('Enter');
await page.waitForTimeout(300);
check('완료 뒤 Enter로 중복 저장 안 함', await page.evaluate(() => window.__mock.exportReq === null));

// 새 영상 편집하기
await click('새 영상 편집하기');
check('처음으로 돌아감', (await page.textContent('.step-label')).includes('1 / 4'));
check('새 영상 편집하기 → CloseVideo', await page.evaluate(() => window.__mock.closed === 1));
check('불러온 영상 표시 없음', await page.$eval('.loaded-card', (e) => e.hidden));

console.log(results.join('\n'));
const errors = logs.filter((l) => l.includes('pageerror') || (l.startsWith('[error]') && !l.includes('404')));
if (errors.length) console.log('브라우저 오류:\n' + errors.join('\n'));
const failed = results.filter((r) => r.startsWith('FAIL')).length + errors.length;
console.log(`${VIDEO}: 통과 ${results.length - failed}/${results.length}`);
await browser.close();
server.close();
process.exit(failed ? 1 : 0);
