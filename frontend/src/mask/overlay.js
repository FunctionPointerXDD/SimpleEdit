import {h} from '../ui/dom.js';

// 이보다 작게 끈 사각형은 실수로 보고 버린다.
const MIN_DRAG_PX = 8;
const clamp01 = (v) => Math.max(0, Math.min(1, v));

// contain으로 그려진 실제 영상 영역(요소 기준 px)
function contentBox(video) {
    const ew = video.clientWidth;
    const eh = video.clientHeight;
    const vw = video.videoWidth || ew;
    const vh = video.videoHeight || eh;
    const scale = Math.min(ew / vw, eh / vh);
    const w = vw * scale;
    const h = vh * scale;
    return {x: (ew - w) / 2, y: (eh - h) / 2, w, h};
}

// 끌어 그린 사각형(비율)과 자동 박스(주황)를 그린다.
export function createMaskOverlay({video, getRects, getAuto, canAdd, onAdd, onBlocked, onStart}) {
    const off = new AbortController();
    const on = (target, type, fn) => target.addEventListener(type, fn, {signal: off.signal});
    const canvas = h('canvas.mask-canvas');
    const ctx2d = canvas.getContext('2d');
    let drag = null;
    let raf = 0;

    function pointIn(ev) {
        const r = canvas.getBoundingClientRect();
        const box = contentBox(video);
        const x = ev.clientX - r.left - box.x;
        const y = ev.clientY - r.top - box.y;
        return {
            inside: x >= 0 && x <= box.w && y >= 0 && y <= box.h,
            x: clamp01(x / box.w),
            y: clamp01(y / box.h),
        };
    }

    const dragRect = () => ({
        x: Math.min(drag.x0, drag.x1),
        y: Math.min(drag.y0, drag.y1),
        w: Math.abs(drag.x1 - drag.x0),
        h: Math.abs(drag.y1 - drag.y0),
    });

    function finish() {
        if (!drag) return;
        const box = contentBox(video);
        const r = dragRect();
        drag = null;
        if (r.w * box.w >= MIN_DRAG_PX && r.h * box.h >= MIN_DRAG_PX) onAdd(r);
        draw();
    }

    function draw() {
        const dpr = window.devicePixelRatio || 1;
        const cw = canvas.clientWidth;
        const ch = canvas.clientHeight;
        if (canvas.width !== Math.round(cw * dpr) || canvas.height !== Math.round(ch * dpr)) {
            canvas.width = Math.round(cw * dpr);
            canvas.height = Math.round(ch * dpr);
        }
        ctx2d.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx2d.clearRect(0, 0, cw, ch);
        const box = contentBox(video);
        const toPx = (r) => [box.x + r.x * box.w, box.y + r.y * box.h, r.w * box.w, r.h * box.h];

        for (const r of getAuto?.(video.currentTime) || []) {
            const [x, y, w, hh] = toPx(r);
            drawMosaicHint(x, y, w, hh);
            ctx2d.lineWidth = 3;
            ctx2d.strokeStyle = '#ff9500';
            ctx2d.strokeRect(x, y, w, hh);
        }
        getRects().forEach((r, i) => {
            const [x, y, w, hh] = toPx(r);
            drawMosaicHint(x, y, w, hh);
            ctx2d.lineWidth = 3;
            ctx2d.strokeStyle = '#ff3b30';
            ctx2d.strokeRect(x, y, w, hh);
            drawBadge(x, y, String(i + 1));
        });
        if (drag) {
            const [x, y, w, hh] = toPx(dragRect());
            ctx2d.setLineDash([8, 6]);
            ctx2d.lineWidth = 3;
            ctx2d.strokeStyle = '#ffcc00';
            ctx2d.strokeRect(x, y, w, hh);
            ctx2d.setLineDash([]);
        }
    }

    function drawMosaicHint(x, y, w, hh) {
        const cell = 12;
        ctx2d.save();
        ctx2d.beginPath();
        ctx2d.rect(x, y, w, hh);
        ctx2d.clip();
        ctx2d.fillStyle = 'rgba(0,0,0,0.45)';
        ctx2d.fillRect(x, y, w, hh);
        ctx2d.fillStyle = 'rgba(255,255,255,0.18)';
        for (let row = 0; y + row * cell < y + hh; row++) {
            for (let xx = x + (row % 2) * cell; xx < x + w; xx += cell * 2) {
                ctx2d.fillRect(xx, y + row * cell, cell, cell);
            }
        }
        ctx2d.restore();
    }

    function drawBadge(x, y, text) {
        const size = 30;
        ctx2d.fillStyle = '#ff3b30';
        ctx2d.fillRect(x, y, size, size);
        ctx2d.fillStyle = '#fff';
        ctx2d.font = 'bold 20px "Malgun Gothic", sans-serif';
        ctx2d.textAlign = 'center';
        ctx2d.textBaseline = 'middle';
        ctx2d.fillText(text, x + size / 2, y + size / 2 + 1);
    }

    on(canvas, 'pointerdown', (ev) => {
        const p = pointIn(ev);
        if (ev.button !== 0 || !p.inside) return;
        if (!canAdd()) {
            onBlocked();
            return;
        }
        onStart?.();
        drag = {x0: p.x, y0: p.y, x1: p.x, y1: p.y};
        canvas.setPointerCapture(ev.pointerId);
        draw();
    });
    on(canvas, 'pointermove', (ev) => {
        const p = pointIn(ev);
        canvas.style.cursor = p.inside ? 'crosshair' : 'default';
        if (!drag) return;
        drag.x1 = p.x;
        drag.y1 = p.y;
        draw();
    });
    on(canvas, 'pointerup', finish);
    on(canvas, 'pointercancel', finish);
    on(video, 'loadedmetadata', draw);
    on(video, 'seeked', draw);
    on(video, 'timeupdate', draw);
    on(video, 'play', () => {
        if (!raf) raf = requestAnimationFrame(tick);
    });

    function tick() {
        draw();
        raf = video.paused ? 0 : requestAnimationFrame(tick);
    }

    const ro = new ResizeObserver(draw);
    ro.observe(canvas);

    return {
        el: canvas,
        redraw: draw,
        destroy() {
            off.abort();
            cancelAnimationFrame(raf);
            ro.disconnect();
        },
    };
}
