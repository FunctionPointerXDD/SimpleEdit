// 재생 막대·시계·앞뒤 버튼은 이어 붙인 시각(0~전체 길이)이고, 구간 끝에 닿으면 다음 구간 시작으로 넘어간다.
// currentTime·seek·playRange는 원본 초, seekOut·playOut은 이어 붙인 초다.
// 구간은 원본에서 겹치지 않아야 한다.
// toOut은 지금 구간(cur)이면 그 끝 시각도 지금 구간으로 보고, 구간 밖이면 지금 구간 쪽 끝으로 붙인다.
// locate는 이어 붙인 시각을 [구간 번호, 원본 초]로 바꾸고, 경계는 뒤 구간의 시작으로 본다.
// playSequence는 원본 구간들을 차례로 재생한다(2단계 "이어서 보기"). 사용자가 옮기거나 멈추면 그친다.

import {h} from './dom.js';
import {formatClock} from '../util/time.js';

// 공유 video를 segments(없으면 min~max) 순서로 이어 재생한다. destroy는 요소만 뗀다.
export function createPlayer({video, duration, min = 0, max = duration, segments = null, fineSteps = true}) {
    const off = new AbortController();
    const on = (target, type, fn) => target.addEventListener(type, fn, {signal: off.signal});
    const segs = segments?.length ? segments : [{start: min, end: max}];
    const outs = [];
    let total = 0;
    for (const s of segs) {
        outs.push(total);
        total += s.end - s.start;
    }

    const stage = h('div.player-stage', {}, video);
    const range = h('input.seek', {type: 'range', min: '0', max: String(total), step: '0.1', value: '0'});
    const clock = h('span.clock');
    const marks = h('div.seek-marks');
    const playBtn = h('button.btn.play', {type: 'button', onclick: togglePlay}, '재생');
    const stepBtn = (label, delta) =>
        h('button.btn.small', {type: 'button', onclick: () => seekOut(currentOut() + delta)}, label);

    const el = h('div.player', {},
        stage,
        h('div.seek-row', {}, h('div.seek-wrap', {}, marks, range), clock),
        h('div.player-controls', {},
            stepBtn('5초 뒤로', -5),
            stepBtn('1초 뒤로', -1),
            fineSteps && stepBtn('0.1초 뒤로', -0.1),
            playBtn,
            fineSteps && stepBtn('0.1초 앞으로', 0.1),
            stepBtn('1초 앞으로', 1),
            stepBtn('5초 앞으로', 5),
        ),
    );

    let dragging = false;
    let stopOut = total;
    let queue = [];
    let cur = 0;
    let raf = 0;
    const endLabel = formatClock(total);

    const indexAt = (t) => segs.findIndex((x) => t >= x.start - 1e-3 && t < x.end - 1e-3);

    function toOut(t, k = cur) {
        const x = segs[k];
        if (t >= x.start - 1e-3 && t <= x.end + 1e-3) return outs[k] + Math.min(Math.max(t - x.start, 0), x.end - x.start);
        const j = indexAt(t);
        if (j >= 0) return outs[j] + t - segs[j].start;
        return t < x.start ? outs[k] : outs[k] + x.end - x.start;
    }

    function locate(o) {
        const t = Math.min(Math.max(o, 0), total);
        let k = segs.length - 1;
        while (k > 0 && t < outs[k] - 1e-3) k--;
        return [k, segs[k].start + Math.min(t - outs[k], segs[k].end - segs[k].start)];
    }

    const currentOut = () => toOut(video.currentTime);

    function seekOut(o) {
        stopOut = total;
        queue = [];
        const [k, t] = locate(o);
        cur = k;
        video.currentTime = t;
        render();
    }

    function seek(t) {
        const k = indexAt(t);
        seekOut(k >= 0 ? outs[k] + t - segs[k].start : t < segs[0].start ? 0 : toOut(t));
    }

    function togglePlay() {
        queue = [];
        if (!video.paused) {
            video.pause();
            return;
        }
        stopOut = total;
        const o = currentOut();
        if (o >= total - 0.05 || indexAt(video.currentTime) < 0) seekOut(o >= total - 0.05 ? 0 : o);
        video.play().catch(() => {});
    }

    function render() {
        const o = currentOut();
        if (!dragging) range.value = String(o);
        clock.textContent = `${formatClock(o)} / ${endLabel}`;
        playBtn.textContent = video.paused ? '재생' : '멈춤';
    }

    function tick() {
        const x = segs[cur];
        if (currentOut() >= stopOut - 1e-3 && queue.length) {
            const [[a, b], ...rest] = queue;
            seek(a);
            stopOut = toOut(b);
            queue = rest;
        } else if (currentOut() >= stopOut - 1e-3) {
            video.pause();
            const [k, t] = locate(stopOut);
            if (k === cur) video.currentTime = t;
        } else if (video.currentTime >= x.end - 1e-3) {
            if (cur + 1 < segs.length) {
                cur++;
                video.currentTime = segs[cur].start;
            } else {
                video.pause();
                video.currentTime = x.end;
            }
        }
        render();
        raf = video.paused ? 0 : requestAnimationFrame(tick);
    }

    function fitIntoRange() {
        const k = indexAt(video.currentTime);
        if (k < 0) seekOut(0);
        else cur = k;
        render();
    }

    on(range, 'input', () => {
        dragging = true;
        seekOut(Number(range.value));
    });
    on(range, 'change', () => { dragging = false; });
    on(video, 'play', () => {
        if (!raf) raf = requestAnimationFrame(tick);
    });
    on(video, 'pause', render);
    on(video, 'seeked', () => {
        const k = indexAt(video.currentTime);
        if (k >= 0) cur = k;
        render();
    });
    on(video, 'loadedmetadata', fitIntoRange);

    video.pause();
    if (video.readyState >= 1) fitIntoRange();
    else render();

    return {
        el,
        video,
        marks,
        total,
        get currentTime() { return video.currentTime; },
        get currentOut() { return currentOut(); },
        seek,
        seekOut,
        pause: () => video.pause(),
        playRange(from, to) {
            seek(from);
            stopOut = toOut(to);
            video.play().catch(() => {});
        },
        playSequence(list) {
            if (!list.length) return;
            seek(list[0][0]);
            stopOut = toOut(list[0][1]);
            queue = list.slice(1);
            video.play().catch(() => {});
        },
        playOut(from, to) {
            seekOut(from);
            stopOut = Math.min(to, total);
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
