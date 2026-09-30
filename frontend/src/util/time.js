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

// now와 같은 날이면 "오늘 13:05", 전날이면 "어제 13:05", 그 밖에는 "9월 27일"
export function formatDay(ms, now = new Date()) {
    const d = new Date(ms);
    const day = (x) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
    const diff = Math.round((day(now) - day(d)) / 86400000);
    const clock = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
    if (diff === 0) return `오늘 ${clock}`;
    if (diff === 1) return `어제 ${clock}`;
    return `${d.getMonth() + 1}월 ${d.getDate()}일`;
}

// 초 단위로 반올림해 "32분 5초", 1분 미만은 "45초"
export function formatWhole(sec) {
    const t = Math.max(0, Math.round(sec));
    const m = Math.floor(t / 60);
    return m > 0 ? `${m}분 ${t % 60}초` : `${t}초`;
}
