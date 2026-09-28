// 초 → "01:02.3" (1시간 이상은 "1:01:02.3")
export function formatClock(sec) {
    const t = Math.max(0, Math.round(sec * 10) / 10);
    const h = Math.floor(t / 3600);
    const m = Math.floor((t % 3600) / 60);
    const s = (t % 60).toFixed(1).padStart(4, '0');
    const mm = String(m).padStart(2, '0');
    return h > 0 ? `${h}:${mm}:${s}` : `${mm}:${s}`;
}

// 길이 → "30.0초", "1분 5.0초"
export function formatLength(sec) {
    const t = Math.max(0, Math.round(sec * 10) / 10);
    const m = Math.floor(t / 60);
    const s = (t % 60).toFixed(1);
    return m > 0 ? `${m}분 ${s}초` : `${s}초`;
}

// 남은 시간 → "약 1분 20초", 모르면 "계산 중"
export function formatRemaining(sec) {
    if (sec == null || sec < 0 || !isFinite(sec)) return '계산 중';
    const t = Math.ceil(sec);
    if (t < 5) return '곧 끝납니다';
    const m = Math.floor(t / 60);
    const s = t % 60;
    if (m === 0) return `약 ${s}초`;
    return s === 0 ? `약 ${m}분` : `약 ${m}분 ${s}초`;
}

// 0.1초 단위로 반올림
export const roundTenth = (sec) => Math.round(sec * 10) / 10;
