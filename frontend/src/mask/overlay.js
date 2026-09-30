// MIN_DRAG_PX보다 작게 끈 것은 클릭으로 보고, 그 자리의 자동 박스를 빼거나 되살린다.
// 가림 표시는 영상 프레임을 저장과 같은 세기로 흐려 그린다. 사각형은 media.BlurSigma(긴 변 20%)다.
// 자동 얼굴은 저장(frame.BlurEllipse)과 같게 박스에 내접한 타원, 직접 그린 곳은 사각형이다.
// 타원은 media.FaceBlurSigma(14%) 세기로, 바깥 ELLIPSE_FEATHER 구간을 smoothstep으로 원본과 섞는다.
// 직접 가림이 꽉 차도 누른 자리에 자동 박스가 있으면 뺄 수 있다. 겹치면 가장 작은 박스를 고른다.

import {h} from '../ui/dom.js';

const MIN_DRAG_PX = 8;
const BLUR_SHARE = 0.2;
const FACE_BLUR_SHARE = 0.14;
const ELLIPSE_FEATHER = 0.25;
const MIN_BLUR_SIGMA = 4;
const clamp01 = (v) => Math.max(0, Math.min(1, v));

// object-fit: contain 비율로 요소 안 실제 영상 영역(px)을 계산한다.
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

// 좌표는 영상 영역 비율. 재생 중엔 rAF, 멈춤 땐 timeupdate·seeked로 다시 그린다.
export function createMaskOverlay({video, getRects, getAuto, onAutoClick, canAdd, onAdd, onBlocked, onStart}) {
    const off = new AbortController();
    const on = (target, type, fn) => target.addEventListener(type, fn, {signal: off.signal});
    const canvas = h('canvas.mask-canvas');
    const ctx2d = canvas.getContext('2d');
    const scratch = document.createElement('canvas');
    const scratchCtx = scratch.getContext('2d');
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

    function finish(clicked) {
        if (!drag) return;
        const box = contentBox(video);
        const r = dragRect();
        const at = {x: drag.x1, y: drag.y1};
        drag = null;
        if (r.w * box.w >= MIN_DRAG_PX && r.h * box.h >= MIN_DRAG_PX) onAdd(r);
        else if (clicked) clickAuto(at);
        draw();
    }

    function autoAt(p) {
        let best = null;
        for (const b of getAuto?.(video.currentTime) || []) {
            const inside = p.x >= b.x && p.x <= b.x + b.w && p.y >= b.y && p.y <= b.y + b.h;
            if (inside && (!best || b.w * b.h < best.w * best.h)) best = b;
        }
        return best;
    }

    function clickAuto(p) {
        const hit = autoAt(p);
        if (hit) onAutoClick?.(hit);
        return !!hit;
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
            if (r.excluded) {
                drawExcluded(x, y, w, hh);
                continue;
            }
            drawBlurHint(box, r, x, y, w, hh, true);
            ctx2d.lineWidth = 3;
            ctx2d.strokeStyle = '#ff9500';
            shapePath(x, y, w, hh, true);
            ctx2d.stroke();
        }
        getRects().forEach((r, i) => {
            const [x, y, w, hh] = toPx(r);
            drawBlurHint(box, r, x, y, w, hh, false);
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

    function drawExcluded(x, y, w, hh) {
        ctx2d.setLineDash([6, 6]);
        ctx2d.lineWidth = 2;
        ctx2d.strokeStyle = 'rgba(200,200,200,0.9)';
        shapePath(x, y, w, hh, true);
        ctx2d.stroke();
        ctx2d.setLineDash([]);
        ctx2d.font = 'bold 16px "Malgun Gothic", sans-serif';
        ctx2d.textAlign = 'left';
        ctx2d.textBaseline = 'top';
        ctx2d.fillStyle = 'rgba(0,0,0,0.6)';
        ctx2d.fillRect(x, y, 44, 22);
        ctx2d.fillStyle = '#fff';
        ctx2d.fillText('빠짐', x + 5, y + 3);
    }

    function shapePath(x, y, w, hh, ellipse) {
        ctx2d.beginPath();
        if (ellipse) ctx2d.ellipse(x + w / 2, y + hh / 2, w / 2, hh / 2, 0, 0, Math.PI * 2);
        else ctx2d.rect(x, y, w, hh);
    }

    function drawBlurHint(box, r, x, y, w, hh, ellipse) {
        const vw = video.videoWidth;
        const vh = video.videoHeight;
        if (!vw || !vh || video.readyState < 2) {
            ctx2d.save();
            shapePath(x, y, w, hh, ellipse);
            ctx2d.fillStyle = 'rgba(128,128,128,0.8)';
            ctx2d.fill();
            ctx2d.restore();
            return;
        }
        const scale = box.w / vw;
        const share = ellipse ? FACE_BLUR_SHARE : BLUR_SHARE;
        const sigma = Math.max(MIN_BLUR_SIGMA * scale, Math.max(w, hh) * share);
        const pad = (sigma * 2) / scale;
        const sx = Math.max(0, r.x * vw - pad);
        const sy = Math.max(0, r.y * vh - pad);
        const sw = Math.min(vw, (r.x + r.w) * vw + pad) - sx;
        const sh = Math.min(vh, (r.y + r.h) * vh + pad) - sy;
        const dest = [box.x + sx * scale, box.y + sy * scale, sw * scale, sh * scale];
        if (!ellipse) {
            ctx2d.save();
            shapePath(x, y, w, hh, false);
            ctx2d.clip();
            ctx2d.filter = `blur(${sigma}px)`;
            ctx2d.drawImage(video, sx, sy, sw, sh, ...dest);
            ctx2d.restore();
            return;
        }
        const dpr = window.devicePixelRatio || 1;
        scratch.width = Math.max(1, Math.ceil(w * dpr));
        scratch.height = Math.max(1, Math.ceil(hh * dpr));
        scratchCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
        scratchCtx.filter = `blur(${sigma}px)`;
        scratchCtx.drawImage(video, sx, sy, sw, sh, dest[0] - x, dest[1] - y, dest[2], dest[3]);
        scratchCtx.filter = 'none';
        scratchCtx.globalCompositeOperation = 'destination-in';
        scratchCtx.translate(w / 2, hh / 2);
        scratchCtx.scale(1, hh / Math.max(w, 1));
        const g = scratchCtx.createRadialGradient(0, 0, 0, 0, 0, w / 2);
        for (let i = 0; i <= 8; i++) {
            const q = 1 - ELLIPSE_FEATHER + (ELLIPSE_FEATHER * i) / 8;
            const t = (1 - q) / ELLIPSE_FEATHER;
            g.addColorStop(q, `rgba(0,0,0,${t * t * (3 - 2 * t)})`);
        }
        g.addColorStop(0, 'rgba(0,0,0,1)');
        scratchCtx.fillStyle = g;
        scratchCtx.beginPath();
        scratchCtx.arc(0, 0, w / 2, 0, Math.PI * 2);
        scratchCtx.fill();
        scratchCtx.globalCompositeOperation = 'source-over';
        ctx2d.drawImage(scratch, x, y, w, hh);
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
            if (!clickAuto(p)) onBlocked();
            draw();
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
    on(canvas, 'pointerup', () => finish(true));
    on(canvas, 'pointercancel', () => finish(false));
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
