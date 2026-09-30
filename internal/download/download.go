// yt-dlp 출력은 SEINFO·SEPROG·SEFILE·SETITLE 접두어로 영상 제목 등과 구분한다.
// parser.weights는 영상·소리 스트림이 전체에서 차지하는 비율이다.
// 크기를 모르면 영상:소리 = 0.9:0.1로 보고, 마지막 3%는 합치기 단계 몫이다.
// 유튜브가 잠시 막거나 서버 오류가 나면(403, 받는 도중 끊김, 500) attempts 순서로 쉬었다 다시 받는다.
// 마지막 시도는 720p다. 1080p 파일만 서버에서 깨진 영상이 있어서다(aUTckJxMazw: 끝 16초가 계속 500).
// 모두 실패하면 ErrBlocked(몇 분 뒤 안내)다. yt-dlp 안쪽 재시도는 5번, 1초부터 8초까지 늘려 가며 쉰다.
// 안쪽 재시도 중에는 StageRetry를 보내 진행률 막대가 멈춘 것처럼 보이지 않게 한다.
// HLS 조각을 못 받으면 건너뛰지 않고 실패시킨다(건너뛰면 중간이 빠진 영상이 조용히 만들어짐).

package download

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"simpleEdit/internal/applog"
	"simpleEdit/internal/tools"
)

type Stage string

const (
	StageStart    Stage = "start"
	StageDownload Stage = "download"
	StageMerge    Stage = "merge"
	StageRetry    Stage = "retry"
)

type Result struct {
	Path    string
	Title   string
	Lowered bool
}

type Progress struct {
	Frac  float64
	Stage Stage
}

var (
	ErrPrivate     = errors.New("비공개 영상")
	ErrUnavailable = errors.New("볼 수 없는 영상")
	ErrAgeLimit    = errors.New("연령 제한 영상")
	ErrLive        = errors.New("생방송 영상")
	ErrBlocked     = errors.New("유튜브가 일시적으로 차단")
	ErrNetwork     = errors.New("네트워크 오류")
	ErrNoSpace     = errors.New("디스크 공간 부족")
	ErrFailed      = errors.New("다운로드 실패")
)

const (
	tagInfo  = "SEINFO"
	tagProg  = "SEPROG"
	tagFile  = "SEFILE"
	tagTitle = "SETITLE"
)

// avc1 maxHeight 이하 + m4a를 우선 고르고, --print·--progress-template로 진행률을 뽑는다.
func Args(videoURL, workDir string, maxHeight int) []string {
	h := strconv.Itoa(maxHeight)
	return []string{
		"--ignore-config", "--no-playlist", "--newline", "--no-mtime", "--no-part",
		"--encoding", "utf-8",
		"-f", "bv*[height<=" + h + "][vcodec^=avc1]+ba[ext=m4a]/" +
			"bv*[height<=" + h + "][ext=mp4]+ba[ext=m4a]/" +
			"b[height<=" + h + "][ext=mp4]/b[ext=mp4]/b",
		"--merge-output-format", "mp4",
		"--ffmpeg-location", tools.BinDir(),
		"--match-filter", "!is_live",
		"--no-simulate", "--progress",
		"--retries", "5", "--retry-sleep", "http:exp=1:8",
		"--abort-on-unavailable-fragments",
		"--print", "before_dl:" + tagInfo + " %(requested_formats.1.format_id|-)s " +
			"%(requested_formats.0.filesize,requested_formats.0.filesize_approx|0)s " +
			"%(requested_formats.1.filesize,requested_formats.1.filesize_approx|0)s",
		"--progress-template", "download:" + tagProg + " %(progress.downloaded_bytes|0)s " +
			"%(progress.total_bytes,progress.total_bytes_estimate|0)s",
		"--print", "before_dl:" + tagTitle + " %(title)s",
		"--print", "after_move:" + tagFile + " %(filepath)s",
		"-o", filepath.Join(workDir, "source.%(ext)s"),
		videoURL,
	}
}

// 일시 오류면 폴더를 비우고 attempts의 대기·화질로 다시 받는다. 그 밖의 실패·취소는 바로 반환
func Download(ctx context.Context, videoURL, workDir string, onProgress func(Progress)) (Result, error) {
	var lastErr error
	for i, at := range attempts {
		if i > 0 {
			applog.Printf("일시 차단·서버 오류로 보여 %v 뒤 %dp 이하로 다시 시도 (%d/%d)", at.wait, at.maxHeight, i+1, len(attempts))
			if onProgress != nil {
				onProgress(Progress{Stage: StageRetry})
			}
			if err := resetDir(workDir); err != nil {
				return Result{}, err
			}
			select {
			case <-time.After(at.wait):
			case <-ctx.Done():
				return Result{}, ctx.Err()
			}
		}
		res, stderr, err := downloadOnce(ctx, videoURL, workDir, at.maxHeight, onProgress)
		if err == nil {
			res.Lowered = at.maxHeight < attempts[0].maxHeight
			return res, nil
		}
		if ctx.Err() != nil || !isTransient(stderr) {
			return res, err
		}
		lastErr = err
	}
	return Result{}, fmt.Errorf("%w: %v", ErrBlocked, lastErr)
}

var attempts = []struct {
	wait      time.Duration
	maxHeight int
}{{0, 1080}, {3 * time.Second, 1080}, {5 * time.Second, 720}}

// stdout을 parser로 읽고, 파일 경로가 안 나왔으면 생방송, 실패는 stderr로 분류
func downloadOnce(ctx context.Context, videoURL, workDir string, maxHeight int, onProgress func(Progress)) (Result, string, error) {
	cmd := tools.Command(ctx, tools.YtDlp, Args(videoURL, workDir, maxHeight)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, "", err
	}
	if err := cmd.Start(); err != nil {
		return Result{}, "", err
	}
	p := newParser(onProgress)
	p.read(stdout)
	err = cmd.Wait()
	stderr := cmd.Stderr()
	switch {
	case ctx.Err() != nil:
		return Result{}, stderr, ctx.Err()
	case err == nil && p.file == "":
		return Result{}, stderr, ErrLive
	case err != nil:
		if c := Classify(stderr); c != nil {
			return Result{}, stderr, c
		}
		return Result{}, stderr, fmt.Errorf("%w: %v", ErrFailed, err)
	}
	return Result{Path: p.file, Title: p.title}, stderr, nil
}

// 403·500, 받는 도중 끊김(… bytes read) 때문에 yt-dlp가 포기한 경우
func isTransient(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "http error 403") || strings.Contains(s, "http error 500") ||
		strings.Contains(s, "bytes read")
}

// 폴더를 통째로 지우고 다시 만든다.
func resetDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

func Update(ctx context.Context) error {
	return tools.Command(ctx, tools.YtDlp, "-U").Run()
}

type parser struct {
	onProgress func(Progress)
	weights    []float64
	part       int
	lastBytes  int64
	lastFrac   float64
	file       string
	title      string
}

func newParser(on func(Progress)) *parser {
	return &parser{onProgress: on, weights: []float64{1}}
}

const mergeShare = 0.03

var unknownWeights = []float64{0.9, 0.1}

func (p *parser) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	p.emit(0, StageStart)
	for sc.Scan() {
		p.line(sc.Text())
	}
	_, _ = io.Copy(io.Discard, r)
}

// SEINFO로 스트림 비중을 정하고, SEPROG 바이트가 줄면 다음 스트림으로 넘긴다.
func (p *parser) line(s string) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, tagInfo+" "):
		f := strings.Fields(s)
		if len(f) < 4 {
			return
		}
		if f[1] == "-" || f[1] == "NA" {
			p.weights = []float64{1}
			return
		}
		a, _ := strconv.ParseFloat(f[2], 64)
		b, _ := strconv.ParseFloat(f[3], 64)
		if a > 0 && b > 0 {
			p.weights = []float64{a / (a + b), b / (a + b)}
		} else {
			p.weights = unknownWeights
		}
	case strings.HasPrefix(s, tagProg+" "):
		f := strings.Fields(s)
		if len(f) < 3 {
			return
		}
		done, _ := strconv.ParseFloat(f[1], 64)
		total, _ := strconv.ParseFloat(f[2], 64)
		if int64(done) < p.lastBytes && p.part < len(p.weights)-1 {
			p.part++
		}
		p.lastBytes = int64(done)
		var partFrac float64
		if total > 0 {
			partFrac = min(1, done/total)
		}
		before := 0.0
		for _, w := range p.weights[:p.part] {
			before += w
		}
		frac := (before + p.weights[p.part]*partFrac) * (1 - mergeShare)
		if p.part == len(p.weights)-1 && partFrac >= 1 {
			p.emit(frac, StageMerge)
			return
		}
		p.emit(frac, StageDownload)
	case strings.HasPrefix(s, tagFile+" "):
		p.file = strings.TrimSpace(strings.TrimPrefix(s, tagFile+" "))
	case strings.HasPrefix(s, tagTitle+" "):
		p.title = strings.TrimSpace(strings.TrimPrefix(s, tagTitle+" "))
	case strings.Contains(s, "Retrying ("):
		p.emit(p.lastFrac, StageRetry)
	}
}

// 이전 값보다 작으면 이전 값으로 올려서 알린다.
func (p *parser) emit(frac float64, st Stage) {
	if frac < p.lastFrac {
		frac = p.lastFrac
	}
	p.lastFrac = frac
	if p.onProgress != nil {
		p.onProgress(Progress{Frac: frac, Stage: st})
	}
}

// stderr를 소문자로 바꿔 알려진 문구를 찾는다. 모르면 nil
func Classify(stderr string) error {
	s := strings.ToLower(stderr)
	has := func(subs ...string) bool {
		for _, x := range subs {
			if strings.Contains(s, x) {
				return true
			}
		}
		return false
	}
	switch {
	case has("no space left", "errno 28", "not enough space"):
		return ErrNoSpace
	case has("private video"):
		return ErrPrivate
	case has("sign in to confirm your age", "age-restricted", "inappropriate for some users"):
		return ErrAgeLimit
	case has("sign in to confirm you", "not a bot", "http error 429"):
		return ErrBlocked
	case has("is_live", "live event", "this live stream", "premieres in"):
		return ErrLive
	case has("video unavailable", "has been removed", "account associated with this video has been terminated",
		"not available in your country", "this video is not available", "members-only", "join this channel"):
		return ErrUnavailable
	case has("getaddrinfo", "failed to resolve", "name resolution", "timed out", "unable to connect",
		"connection refused", "connection reset", "network is unreachable", "unable to download webpage",
		"urlopen error", "remote end closed", "http error 5"):
		return ErrNetwork
	}
	return nil
}
