import {h} from './dom.js';
import {formatClock} from '../util/time.js';

// 공유 video를 쓰는 플레이어. destroy해도 영상은 유지
export function createPlayer({video, duration, min = 0, max = duration, fineSteps = true}) {
    const off = new AbortController();
    const on = (target, type, fn) => target.addEventListener(type, fn, {signal: off.signal});

    const stage = h('div.player-stage', {}, video);
    const range = h('input.seek', {type: 'range', min: String(min), max: String(max), step: '0.1', value: String(min)});
    const clock = h('span.clock');
    const marks = h('div.seek-marks');
    const playBtn = h('button.btn.play', {type: 'button', onclick: togglePlay}, '재생');
    const stepBtn = (label, delta) =>
        h('button.btn.small', {type: 'button', onclick: () => seek(video.currentTime + delta)}, label);

    const el = h('div.player', {},
        stage,
        h('div.seek-wrap', {}, marks, range),
        h('div.player-controls', {},
            stepBtn('5초 뒤로', -5),
            fineSteps && stepBtn('0.1초 뒤로', -0.1),
            playBtn,
            fineSteps && stepBtn('0.1초 앞으로', 0.1),
            stepBtn('5초 앞으로', 5),
            clock,
        ),
    );

    let dragging = false;
    let stopAt = max;
    let raf = 0;
    const endLabel = formatClock(max);

    const clampTime = (t) => Math.max(min, Math.min(max, t));

    function seek(t) {
        stopAt = max;
        video.currentTime = clampTime(t);
        render();
    }

    function togglePlay() {
        if (!video.paused) {
            video.pause();
            return;
        }
        stopAt = max;
        if (video.currentTime >= max - 0.05 || video.currentTime < min) video.currentTime = min;
        video.play().catch(() => {});
    }

    function render() {
        const t = video.currentTime;
        if (!dragging) range.value = String(t);
        clock.textContent = `${formatClock(t)} / ${endLabel}`;
        playBtn.textContent = video.paused ? '재생' : '멈춤';
    }

    function tick() {
        if (video.currentTime >= stopAt) {
            video.pause();
            video.currentTime = stopAt;
        }
        render();
        raf = video.paused ? 0 : requestAnimationFrame(tick);
    }

    function fitIntoRange() {
        if (video.currentTime < min || video.currentTime > max) video.currentTime = min;
        render();
    }

    on(range, 'input', () => {
        dragging = true;
        stopAt = max;
        video.currentTime = Number(range.value);
        render();
    });
    on(range, 'change', () => { dragging = false; });
    on(video, 'play', () => {
        if (!raf) raf = requestAnimationFrame(tick);
    });
    on(video, 'pause', render);
    on(video, 'seeked', render);
    on(video, 'loadedmetadata', fitIntoRange);

    video.pause();
    if (video.readyState >= 1) fitIntoRange();
    else render();

    return {
        el,
        video,
        marks,
        get currentTime() { return video.currentTime; },
        seek,
        pause: () => video.pause(),
        playRange(from, to) {
            video.currentTime = clampTime(from);
            stopAt = clampTime(to);
            video.play().catch(() => {});
        },
        destroy() {
            off.abort();
            cancelAnimationFrame(raf);
            video.pause();
            video.remove();
        },
    };
}
