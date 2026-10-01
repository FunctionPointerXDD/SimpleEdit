package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestChunkRange(t *testing.T) {
	const size, limit = 100, 30
	tests := []struct {
		header     string
		start, end int64
		ok         bool
	}{
		{"", 0, 29, true},
		{"bytes=0-", 0, 29, true},
		{"bytes=10-19", 10, 19, true},
		{"bytes=90-", 90, 99, true},
		{"bytes=95-200", 95, 99, true},
		{"bytes=-5", 95, 99, true},
		{"bytes=-500", 0, 29, true},
		{"bytes=0-5,10-20", 0, 5, true},
		{"bytes=100-", 0, 0, false},
		{"bytes=20-10", 0, 0, false},
		{"bytes=abc", 0, 0, false},
		{"bytes=-0", 0, 0, false},
	}
	for _, tt := range tests {
		s, e, ok := chunkRange(tt.header, size, limit)
		if ok != tt.ok || ok && (s != tt.start || e != tt.end) {
			t.Errorf("%q = %d-%d %v, want %d-%d %v", tt.header, s, e, ok, tt.start, tt.end, tt.ok)
		}
	}
}

func TestServeVideoCapsChunk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v.mp4")
	data := make([]byte, maxRangeChunk*3)
	for i := range data {
		data[i] = byte(i)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{preview: path}

	req := httptest.NewRequest(http.MethodGet, "/video/current?v=1", nil)
	req.Header.Set("Range", "bytes=0-")
	rec := httptest.NewRecorder()
	a.serveVideo(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.Len() != maxRangeChunk {
		t.Fatalf("상태 %d, 본문 %d바이트 (최대 %d)", rec.Code, rec.Body.Len(), maxRangeChunk)
	}
	want := "bytes 0-4194303/12582912"
	if got := rec.Header().Get("Content-Range"); got != want {
		t.Errorf("Content-Range %q, want %q", got, want)
	}

	req.Header.Set("Range", "bytes=12582900-")
	rec = httptest.NewRecorder()
	a.serveVideo(rec, req)
	if rec.Body.Len() != 12 || rec.Body.Bytes()[0] != data[12582900] {
		t.Errorf("끝부분 %d바이트", rec.Body.Len())
	}
}
