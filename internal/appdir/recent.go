// 유튜브에서 받은 원본은 work\날짜-시각\ 폴더에 두고, 같은 폴더의 meta.json에 주소·제목·길이를 적는다.
// meta.json이 있고 원본 파일이 남아 있는 폴더만 "최근 받은 영상"이다(로컬 열기 폴더는 meta가 없음).
// 폴더는 앱 시작 때 CleanOld로 KeepDays일 지나면 지워지므로 목록도 그만큼만 남는다.

package appdir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	metaName = "meta.json"
	KeepDays = 7
)

type Meta struct {
	URL      string    `json:"url"`
	Title    string    `json:"title"`
	Duration float64   `json:"duration"`
	File     string    `json:"file"`
	SavedAt  time.Time `json:"savedAt"`
}

type Recent struct {
	ID   string
	Path string
	Meta Meta
}

// 폴더 dir에 meta.json을 쓴다. File은 폴더 안 파일 이름만 둔다.
func WriteMeta(dir string, m Meta) error {
	m.File = filepath.Base(m.File)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, metaName), data, 0o644)
}

// work 아래 폴더 중 meta.json이 읽히고 원본이 있는 것만 받은 시각 최신순으로
func RecentDownloads() []Recent {
	entries, err := os.ReadDir(Work())
	if err != nil {
		return nil
	}
	var out []Recent
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(Work(), e.Name())
		data, err := os.ReadFile(filepath.Join(dir, metaName))
		if err != nil {
			continue
		}
		var m Meta
		if json.Unmarshal(data, &m) != nil || m.File == "" || m.URL == "" {
			continue
		}
		path := filepath.Join(dir, filepath.Base(m.File))
		if st, err := os.Stat(path); err != nil || st.Size() == 0 {
			continue
		}
		out = append(out, Recent{ID: e.Name(), Path: path, Meta: m})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Meta.SavedAt.After(out[j].Meta.SavedAt) })
	return out
}

// RecentDownloads 중 주소가 같은 가장 최근 것
func FindDownload(url string) (Recent, bool) {
	for _, r := range RecentDownloads() {
		if r.Meta.URL == url {
			return r, true
		}
	}
	return Recent{}, false
}

// RecentDownloads 목록에 있는 폴더 이름만 받아, 임의 경로를 열지 못하게 한다.
func RecentByID(id string) (Recent, bool) {
	for _, r := range RecentDownloads() {
		if r.ID == id {
			return r, true
		}
	}
	return Recent{}, false
}
