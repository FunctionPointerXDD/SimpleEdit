package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

// 프론트로 보내는 진행률 이벤트 이름
const (
	evDownloadProgress = "download:progress"
	evExportProgress   = "export:progress"
)

// 시간·길이 기준값
const (
	workMaxAge       = 7 * 24 * time.Hour
	updateTimeout    = 2 * time.Minute
	updateWaitMax    = 20 * time.Second
	shutdownWaitMax  = 5 * time.Second
	progressInterval = 250 * time.Millisecond
	minClipSeconds   = 1.0
)

// VideoInfo는 불러온 영상 정보다.
type VideoInfo struct {
	URL      string  `json:"url"`
	Duration float64 `json:"duration"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Name     string  `json:"name"`
}

// ExportRequest의 사각형은 영상 영역 기준 0~1 비율.
type ExportRequest struct {
	Start    float64      `json:"start"`
	End      float64      `json:"end"`
	Rects    []media.Rect `json:"rects"`
	FileName string       `json:"fileName"`
	Auto     []string     `json:"auto"`
}

// ProgressEvent의 Remaining은 남은 초, 모르면 -1.
type ProgressEvent struct {
	Percent   float64 `json:"percent"`
	Remaining float64 `json:"remaining"`
	Stage     string  `json:"stage"`
}

// App은 프론트에 노출되는 바인딩 메서드를 가진다.
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
	auto       map[string]automask.Result

	detMu   sync.Mutex
	faceDet *detect.FaceDetector
}

func NewApp() *App {
	return &App{updateDone: make(chan struct{}), auto: map[string]automask.Result{}}
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

// updateTools는 yt-dlp를 갱신한다. 실패해도 무시.
func (a *App) updateTools() {
	defer close(a.updateDone)
	ctx, cancel := context.WithTimeout(a.ctx, updateTimeout)
	defer cancel()
	if err := download.Update(ctx); err != nil {
		applog.Printf("yt-dlp 업데이트 실패 (무시): %v", err)
	}
}

// beforeClose는 작업 중 닫기를 확인한다. true면 유지.
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

// shutdown은 미완성 파일 정리가 끝날 때까지 기다린다.
func (a *App) shutdown(ctx context.Context) {
	if a.jobs.CancelAndWait(shutdownWaitMax) {
		a.closeDetectors()
	} else {
		applog.Printf("종료 전 작업 정리가 시간 안에 끝나지 않음 (검출기 해제 생략)")
	}
	applog.Printf("===== 종료")
	applog.Close()
}

// Download는 유튜브 영상을 받아 미리보기를 준비한다.
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
	path, err := download.Download(ctx, videoURL, workDir, func(p download.Progress) {
		emit(p.Frac, string(p.Stage))
	})
	if err != nil {
		os.RemoveAll(workDir)
		return VideoInfo{}, downloadError(err)
	}
	info, err := a.load(ctx, path, workDir, emit)
	if err != nil {
		os.RemoveAll(workDir)
		return VideoInfo{}, downloadError(err)
	}
	info.Name = "유튜브 영상"
	return info, nil
}

// OpenLocal은 내 컴퓨터 영상을 연다. 취소하면 빈 값.
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

// load는 정보를 읽고 필요하면 미리보기 파일을 만든다.
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
	a.auto = map[string]automask.Result{}
	a.version++
	return VideoInfo{
		URL:      "/video/current?v=" + strconv.Itoa(a.version),
		Duration: info.Duration,
		Width:    info.Width,
		Height:   info.Height,
	}, nil
}

// CloseVideo는 편집하던 영상을 닫는다.
func (a *App) CloseVideo() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.source, a.preview, a.info = "", "", media.Info{}
	a.auto = map[string]automask.Result{}
	a.lastOutput = ""
	applog.Printf("영상 닫음")
}

// OutputFolder는 저장 폴더. 처음엔 바탕화면
func (a *App) OutputFolder() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.OutputDirOrDesktop()
}

// DefaultFileName은 확장자 없는 기본 파일 이름이다.
func (a *App) DefaultFileName() string {
	return appdir.DefaultFileName(time.Now())
}

// ChooseOutputFolder는 저장 폴더를 고르고 기억한다.
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

// Export는 자르고 가린 영상을 저장하고 경로를 반환한다.
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
	var auto [][]media.Rect
	if len(req.Auto) > 0 {
		if auto, err = a.autoFrames(req.Auto, start, end); err != nil {
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
	out := appdir.OutputPath(outDir, name)
	rects := media.RectsToPixels(req.Rects, info.Width, info.Height)
	applog.Printf("저장: %.2f~%.2f초, 사각형 %v, 자동 %v → %s", start, end, rects, req.Auto, out)

	emit := a.progressEmitter(evExportProgress)
	emit(0, "export")
	progress := func(f float64) { emit(f, "export") }
	if auto != nil {
		err = exportAuto(ctx, pipeSpec(source, info, start, end), out, req.Rects, auto, progress)
	} else {
		err = media.Export(ctx, media.ExportSpec{
			Source: source, Output: out, Start: start, Duration: end - start, Rects: rects,
		}, progress)
	}
	if err != nil {
		return "", exportError(err)
	}
	a.mu.Lock()
	a.lastOutput = out
	a.mu.Unlock()
	return out, nil
}

// estimateSize는 저장 크기를 넉넉히 어림한다.
func estimateSize(info media.Info, seconds float64) int64 {
	bps := info.BitRate
	if bps <= 0 {
		bps = 8_000_000
	}
	return int64(float64(bps)/8*seconds*1.5) + 50<<20
}

// Cancel은 진행 중인 다운로드·저장을 멈춘다.
func (a *App) Cancel() {
	applog.Printf("사용자가 취소함")
	a.jobs.Cancel()
}

// OpenOutputFolder는 저장한 파일을 선택해 폴더를 연다.
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

// progressEmitter는 진행률을 250ms 간격으로 보낸다.
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

// serveVideo는 미리보기 파일을 Range 지원으로 내보낸다.
func (a *App) serveVideo(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/video/current" {
		http.NotFound(w, r)
		return
	}
	a.mu.Lock()
	path := a.preview
	a.mu.Unlock()
	if path == "" {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		applog.Printf("미리보기 파일 열기 실패: %v", err)
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "video.mp4", st.ModTime(), f)
}
