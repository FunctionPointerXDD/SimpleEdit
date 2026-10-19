// ExportRequest.Rects는 레터박스를 뺀 영상 영역 기준 0~1 비율 좌표와 원본 기준 켜는 시각(초)이다.
// 시각은 저장 구간으로 자르고 프레임 번호(round(초×fps))로 바꾼다. 가림이 있으면 Go 파이프로 저장한다.
// End ≤ Start면 구간 전체로 본다(잘못 와도 덜 가리는 쪽으로 틀리지 않게).
// ProgressEvent.Remaining은 남은 초이고, 아직 모르면 -1이다.
// 유튜브 영상 이름은 yt-dlp가 알려 준 제목, 없으면 youtubeName이다. 받은 원본은 7일 보관(appdir.KeepDays).
// VideoInfo.Lowered는 1080p를 못 받아 720p로 받았다는 뜻이다.
// ExportRequest.Overwrite가 true이고 이름·저장 폴더가 마지막 저장과 같으면 그 파일을 새 내용으로 바꾼다.
// 그 밖에는 겹치지 않는 새 이름(이름_2.mp4 등)으로 저장한다. 다른 기존 파일은 덮어쓰지 않는다.
// 덮어쓸 때는 옆에 임시 파일로 먼저 저장하고 성공하면 바꿔 넣어, 실패·취소해도 기존 파일이 남는다.
// 시각·길이(start, end, duration)는 모두 초 단위다. ProgressEvent.Percent는 0~100이다.
// ProgressEvent.Stage: 다운로드는 start·download·merge·convert, 저장은 export, 분석은 analyze

package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"simpleEdit/internal/appdir"
	"simpleEdit/internal/applog"
	"simpleEdit/internal/automask"
	"simpleEdit/internal/detect"
	"simpleEdit/internal/download"
	"simpleEdit/internal/job"
	"simpleEdit/internal/media"
	"simpleEdit/internal/tools"
)

const youtubeName = "유튜브 영상"

const (
	evDownloadProgress = "download:progress"
	evExportProgress   = "export:progress"
)

const (
	workMaxAge       = appdir.KeepDays * 24 * time.Hour
	updateTimeout    = 2 * time.Minute
	updateWaitMax    = 20 * time.Second
	shutdownWaitMax  = 5 * time.Second
	progressInterval = 250 * time.Millisecond
	minClipSeconds   = 1.0
)

type VideoInfo struct {
	URL      string  `json:"url"`
	Duration float64 `json:"duration"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Name     string  `json:"name"`
	Reused   bool    `json:"reused"`
	Lowered  bool    `json:"lowered"`
}

type RecentVideo struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Duration float64 `json:"duration"`
	SavedAt  int64   `json:"savedAt"`
}

type ExportRequest struct {
	Start     float64          `json:"start"`
	End       float64          `json:"end"`
	Rects     []media.MaskRect `json:"rects"`
	FileName  string           `json:"fileName"`
	Auto      []string         `json:"auto"`
	Mute      bool             `json:"mute"`
	Exclude   map[string][]int `json:"exclude"`
	Overwrite bool             `json:"overwrite"`
}

type ProgressEvent struct {
	Percent   float64 `json:"percent"`
	Remaining float64 `json:"remaining"`
	Stage     string  `json:"stage"`
}

type App struct {
	ctx        context.Context
	jobs       job.Manager
	updateDone chan struct{}

	mu         sync.Mutex
	settings   appdir.Settings
	source     string
	preview    string
	info       media.Info
	version    int
	lastOutput string
	lastName   string
	lastDir    string
	auto       map[string]*automask.Coverage
	paths      map[int]*automask.Path
	nextPath   int

	detMu    sync.Mutex
	faceDet  *detect.FaceDetector
	plateDet *detect.PlateDetector
	tracker  *detect.Tracker
}

func NewApp() *App {
	return &App{updateDone: make(chan struct{}), auto: map[string]*automask.Coverage{}, paths: map[int]*automask.Path{}}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := applog.Open(appdir.Logs()); err != nil {
		println("로그 파일을 열지 못함:", err.Error())
	}
	applog.Printf("===== 시작 (도구 폴더: %s)", tools.BinDir())
	if n := appdir.CleanOld(appdir.Work(), workMaxAge, time.Now()); n > 0 {
		applog.Printf("오래된 작업 파일 %d개 삭제", n)
	}
	a.settings = appdir.LoadSettings()
	go a.updateTools()
}

// 제한 시간 안에 yt-dlp -U를 돌리고, 실패는 로그만 남긴 뒤 updateDone을 닫는다.
func (a *App) updateTools() {
	defer close(a.updateDone)
	ctx, cancel := context.WithTimeout(a.ctx, updateTimeout)
	defer cancel()
	if err := download.Update(ctx); err != nil {
		applog.Printf("yt-dlp 업데이트 실패 (무시): %v", err)
	}
}

// 작업 중일 때만 묻고, "닫기"가 아니면 true를 돌려 창 닫기를 막는다.
func (a *App) beforeClose(ctx context.Context) bool {
	if !a.jobs.Running() {
		return false
	}
	res, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "작업 중",
		Message:       "아직 작업 중입니다. 지금 닫으면 작업이 취소됩니다. 닫을까요?",
		Buttons:       []string{"닫기", "계속 작업"},
		DefaultButton: "계속 작업",
		CancelButton:  "계속 작업",
	})
	if err != nil {
		return false
	}
	return res != "닫기" && res != "Yes"
}

// 작업을 취소하고 기다린 뒤, 제때 끝났을 때만 검출기를 해제한다.
func (a *App) shutdown(ctx context.Context) {
	if a.jobs.CancelAndWait(shutdownWaitMax) {
		a.closeDetectors()
	} else {
		applog.Printf("종료 전 작업 정리가 시간 안에 끝나지 않음 (검출기 해제 생략)")
	}
	applog.Printf("===== 종료")
	applog.Close()
}

// 7일 안에 받은 같은 주소가 있으면 그 파일을 연다(실패하면 새로 받음). 아니면 갱신 대기 → 받기 → load
func (a *App) Download(rawURL string) (VideoInfo, error) {
	videoURL, err := download.NormalizeURL(rawURL)
	if err != nil {
		return VideoInfo{}, downloadError(err)
	}
	ctx, done, err := a.jobs.Begin(a.ctx)
	if err != nil {
		return VideoInfo{}, downloadError(err)
	}
	defer done()

	emit := a.progressEmitter(evDownloadProgress)
	emit(0, string(download.StageStart))
	if r, ok := appdir.FindDownload(videoURL); ok {
		applog.Printf("받아 둔 원본 재사용: %s → %s", videoURL, r.Path)
		info, err := a.openDownloaded(ctx, r, emit)
		if err == nil {
			return info, nil
		}
		if ctx.Err() != nil {
			return VideoInfo{}, downloadError(ctx.Err())
		}
		applog.Printf("받아 둔 원본을 열지 못해 새로 받음: %v", err)
	}
	select {
	case <-a.updateDone:
	case <-time.After(updateWaitMax):
	case <-ctx.Done():
		return VideoInfo{}, downloadError(ctx.Err())
	}

	workDir, err := appdir.NewWorkDir(time.Now())
	if err != nil {
		return VideoInfo{}, downloadError(err)
	}
	applog.Printf("다운로드: %s → %s", videoURL, workDir)
	res, err := download.Download(ctx, videoURL, workDir, func(p download.Progress) {
		emit(p.Frac, string(p.Stage))
	})
	if err != nil {
		os.RemoveAll(workDir)
		return VideoInfo{}, downloadError(err)
	}
	info, err := a.load(ctx, res.Path, workDir, emit)
	if err != nil {
		os.RemoveAll(workDir)
		return VideoInfo{}, downloadError(err)
	}
	info.Name, info.Lowered = cmp.Or(res.Title, youtubeName), res.Lowered
	meta := appdir.Meta{URL: videoURL, Title: info.Name, Duration: info.Duration, File: res.Path, SavedAt: time.Now()}
	if err := appdir.WriteMeta(workDir, meta); err != nil {
		applog.Printf("받은 영상 기록 실패: %v", err)
	}
	return info, nil
}

// 최근 받은 영상 목록을 화면용(받은 시각은 유닉스 밀리초)으로 바꾼다.
func (a *App) RecentVideos() []RecentVideo {
	out := []RecentVideo{}
	for _, r := range appdir.RecentDownloads() {
		out = append(out, RecentVideo{ID: r.ID, Title: r.Meta.Title, Duration: r.Meta.Duration, SavedAt: r.Meta.SavedAt.UnixMilli()})
	}
	return out
}

// 목록에 있는 id만 받아 그 폴더의 원본을 다시 받지 않고 연다.
func (a *App) OpenRecent(id string) (VideoInfo, error) {
	r, ok := appdir.RecentByID(id)
	if !ok {
		return VideoInfo{}, userError(msgRecentGone, fmt.Errorf("최근 영상 없음: %q", id))
	}
	ctx, done, err := a.jobs.Begin(a.ctx)
	if err != nil {
		return VideoInfo{}, downloadError(err)
	}
	defer done()
	emit := a.progressEmitter(evDownloadProgress)
	emit(0, string(download.StageStart))
	info, err := a.openDownloaded(ctx, r, emit)
	switch {
	case err == nil:
		return info, nil
	case ctx.Err() != nil:
		return VideoInfo{}, downloadError(ctx.Err())
	}
	return VideoInfo{}, userError(msgRecentGone, err)
}

// 받아 둔 폴더를 작업 폴더로 삼아 load하고 제목과 재사용 표시를 붙인다.
func (a *App) openDownloaded(ctx context.Context, r appdir.Recent, emit func(float64, string)) (VideoInfo, error) {
	info, err := a.load(ctx, r.Path, filepath.Dir(r.Path), emit)
	if err != nil {
		return VideoInfo{}, err
	}
	info.Name, info.Reused = cmp.Or(r.Meta.Title, youtubeName), true
	return info, nil
}

// 파일 대화상자에서 고르지 않으면 빈 값, 고르면 새 작업 폴더에서 load한다.
func (a *App) OpenLocal() (VideoInfo, error) {
	if a.jobs.Running() {
		return VideoInfo{}, userError(msgBusy, nil)
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "영상 파일 고르기",
		Filters: []runtime.FileFilter{
			{DisplayName: "동영상 파일 (*.mp4, *.mov, *.m4v, *.mkv)", Pattern: "*.mp4;*.mov;*.m4v;*.mkv"},
		},
	})
	if err != nil || path == "" {
		return VideoInfo{}, nil
	}
	ctx, done, err := a.jobs.Begin(a.ctx)
	if err != nil {
		return VideoInfo{}, downloadError(err)
	}
	defer done()

	workDir, err := appdir.NewWorkDir(time.Now())
	if err != nil {
		return VideoInfo{}, userError(msgOpenFail, err)
	}
	info, err := a.load(ctx, path, workDir, a.progressEmitter(evDownloadProgress))
	if err != nil {
		os.RemoveAll(workDir)
		if errors.Is(err, context.Canceled) || errors.Is(err, tools.ErrMissing) {
			return VideoInfo{}, downloadError(err)
		}
		return VideoInfo{}, userError(msgOpenFail, err)
	}
	info.Name = filepath.Base(path)
	return info, nil
}

// ffprobe로 읽고, WebView가 못 트는 코덱이면 H.264 미리보기를 만든다.
func (a *App) load(ctx context.Context, path, workDir string, emit func(float64, string)) (VideoInfo, error) {
	info, err := media.Probe(ctx, path)
	if err != nil {
		return VideoInfo{}, err
	}
	applog.Printf("영상 정보: %s %+v", path, info)
	preview := path
	if !info.PlayableInWebView() {
		preview = filepath.Join(workDir, "preview.mp4")
		emit(0, "convert")
		if err := media.MakePreview(ctx, path, preview, info.Duration, func(f float64) {
			emit(f, "convert")
		}); err != nil {
			return VideoInfo{}, err
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.source, a.preview, a.info = path, preview, info
	a.auto = map[string]*automask.Coverage{}
	a.paths = map[int]*automask.Path{}
	a.lastOutput, a.lastName, a.lastDir = "", "", ""
	a.version++
	return VideoInfo{
		URL:      "/video/current?v=" + strconv.Itoa(a.version),
		Duration: info.Duration,
		Width:    info.Width,
		Height:   info.Height,
	}, nil
}

// 경로·정보·자동 분석 결과만 비운다. 파일은 지우지 않는다.
func (a *App) CloseVideo() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.source, a.preview, a.info = "", "", media.Info{}
	a.auto = map[string]*automask.Coverage{}
	a.paths = map[int]*automask.Path{}
	a.lastOutput, a.lastName, a.lastDir = "", "", ""
	applog.Printf("영상 닫음")
}

func (a *App) OutputFolder() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.OutputDirOrDesktop()
}

func (a *App) DefaultFileName() string {
	return appdir.DefaultFileName(time.Now())
}

// 폴더 대화상자에서 고른 경로를 settings.json에 쓴다. 취소하면 현재 폴더
func (a *App) ChooseOutputFolder() (string, error) {
	current := a.OutputFolder()
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "저장할 폴더 고르기",
		DefaultDirectory:     current,
		CanCreateDirectories: true,
	})
	if err != nil || dir == "" {
		return current, nil
	}
	a.mu.Lock()
	a.settings.OutputDir = dir
	s := a.settings
	a.mu.Unlock()
	if err := appdir.SaveSettings(s); err != nil {
		applog.Printf("설정 저장 실패: %v", err)
	}
	applog.Printf("저장 폴더 변경: %s", dir)
	return dir, nil
}

// 자동 박스가 있으면 프레임 파이프(Go 블러), 없으면 ffmpeg 필터로 저장
func (a *App) Export(req ExportRequest) (string, error) {
	a.mu.Lock()
	source, info, outDir := a.source, a.info, a.settings.OutputDirOrDesktop()
	a.mu.Unlock()
	if source == "" {
		return "", userError(msgNoVideo, nil)
	}
	start, end, err := clipRange(info, req.Start, req.End)
	if err != nil {
		return "", err
	}
	if len(req.Rects) > media.MaxRects {
		return "", userError(msgTooManyRects, nil)
	}
	name, err := appdir.CleanFileName(req.FileName)
	if err != nil {
		return "", fileNameError(err)
	}
	var auto []automask.AutoLayer
	if len(req.Auto) > 0 {
		if auto, err = a.autoFrames(req.Auto, req.Exclude, start, end); err != nil {
			return "", userError(msgAutoStale, err)
		}
	}

	ctx, done, err := a.jobs.Begin(a.ctx)
	if err != nil {
		return "", exportError(err)
	}
	defer done()

	if free := appdir.FreeBytes(outDir); free >= 0 && free < estimateSize(info, end-start) {
		return "", userError(msgNoSpace, errors.New("남은 공간 "+strconv.FormatInt(free, 10)))
	}
	out, replace := a.exportTarget(outDir, name, req.Overwrite)
	write := out
	if replace {
		write = savingPath(out)
		os.Remove(write)
	}
	a.mu.Lock()
	paths := maps.Clone(a.paths)
	a.mu.Unlock()
	manual, err := manualRects(req.Rects, start, end, info.FPS, paths)
	if err != nil {
		return "", userError(msgTrackStale, err)
	}
	applog.Printf("저장: %.2f~%.2f초, 사각형 %+v, 자동 %v, 무음 %v, 덮어쓰기 %v → %s", start, end, manual, req.Auto, req.Mute, replace, out)

	emit := a.progressEmitter(evExportProgress)
	emit(0, "export")
	progress := func(f float64) { emit(f, "export") }
	if auto != nil || len(manual) > 0 {
		spec := pipeSpec(source, info, start, end)
		spec.Mute = req.Mute
		err = media.ProcessFrames(ctx, spec, write, automask.BlurFunc(manual, auto), progress)
	} else {
		err = media.Export(ctx, media.ExportSpec{
			Source: source, Output: write, Start: start, Duration: end - start, Mute: req.Mute,
		}, progress)
	}
	if err != nil {
		return "", exportError(err)
	}
	if replace {
		if err := os.Rename(write, out); err != nil {
			os.Remove(write)
			return "", userError(msgOverwriteFail, err)
		}
	}
	tools.NotifyShell(out, !replace)
	a.mu.Lock()
	a.lastOutput, a.lastName, a.lastDir = out, name, outDir
	a.mu.Unlock()
	return out, nil
}

// 덮어쓰기를 원하고 이름·폴더가 마지막 저장과 같으면 그 파일(true), 아니면 겹치지 않는 새 경로
func (a *App) exportTarget(outDir, name string, overwrite bool) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if overwrite && a.lastOutput != "" && name == a.lastName &&
		strings.EqualFold(filepath.Clean(outDir), filepath.Clean(a.lastDir)) {
		return a.lastOutput, true
	}
	return appdir.OutputPath(outDir, name), false
}

// 이름.mp4 옆의 임시 저장 경로. 확장자가 .mp4여야 ffmpeg가 형식을 안다.
func savingPath(out string) string {
	return strings.TrimSuffix(out, filepath.Ext(out)) + ".saving.mp4"
}

// 비트레이트(모르면 8Mbps) × 길이 × 1.5 + 50MB
func estimateSize(info media.Info, seconds float64) int64 {
	bps := info.BitRate
	if bps <= 0 {
		bps = 8_000_000
	}
	return int64(float64(bps)/8*seconds*1.5) + 50<<20
}

func (a *App) Cancel() {
	applog.Printf("사용자가 취소함")
	a.jobs.Cancel()
}

// 마지막 저장 파일을 탐색기에서 선택한다. 없으면 저장 폴더를 연다.
func (a *App) OpenOutputFolder() {
	a.mu.Lock()
	out := a.lastOutput
	if out == "" {
		out = a.settings.OutputDirOrDesktop()
	}
	a.mu.Unlock()
	if err := tools.Reveal(out); err != nil {
		applog.Printf("폴더 열기 실패: %v", err)
	}
}

// 단계가 바뀌면 ETA를 새로 재고, 250ms 간격으로만 이벤트를 보낸다.
func (a *App) progressEmitter(name string) func(frac float64, stage string) {
	var (
		eta       = job.NewETA()
		throttle  = job.NewThrottle(progressInterval)
		lastStage string
	)
	return func(frac float64, stage string) {
		if stage != lastStage {
			eta = job.NewETA()
			throttle = job.NewThrottle(progressInterval)
			lastStage = stage
		}
		if !throttle.Allow(frac) {
			return
		}
		runtime.EventsEmit(a.ctx, name, ProgressEvent{
			Percent:   min(100, frac*100),
			Remaining: eta.Remaining(frac),
			Stage:     stage,
		})
	}
}

// 켜는 시각을 저장 구간 프레임 [First, End)로 바꾸고, 추적 가림은 그 구간의 경로 상자를 붙인다.
func manualRects(rs []media.MaskRect, start, end, fps float64, paths map[int]*automask.Path) ([]automask.ManualRect, error) {
	total := automask.FrameAt(end-start, fps)
	base := automask.FrameAt(start, fps)
	var manual []automask.ManualRect
	for _, r := range rs {
		first, last := 0, total
		if r.End > r.Start {
			first = max(0, automask.FrameAt(r.Start-start, fps))
			last = min(total, automask.FrameAt(r.End-start, fps))
		}
		m := automask.ManualRect{Rect: r.Box()}
		if r.Track != 0 {
			p, ok := paths[r.Track]
			if !ok {
				return nil, fmt.Errorf("추적 %d 없음", r.Track)
			}
			first = max(first, p.First-base)
			last = min(last, p.First+len(p.Boxes)-base)
			if last > first {
				m.Path = p.Boxes[first+base-p.First : last+base-p.First]
			}
		}
		if last > first {
			m.First, m.End = first, last
			manual = append(manual, m)
		}
	}
	return manual, nil
}
