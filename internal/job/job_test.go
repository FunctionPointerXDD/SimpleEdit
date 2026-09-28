package job

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManagerSingleJob(t *testing.T) {
	var m Manager
	ctx, done, err := m.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Begin(context.Background()); !errors.Is(err, ErrBusy) {
		t.Fatalf("두 번째 작업 err = %v, want ErrBusy", err)
	}
	m.Cancel()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("Cancel 후에도 ctx가 끝나지 않음")
	}
	if !m.Running() {
		t.Error("done 전까지는 실행 중으로 봐야 함")
	}
	done()
	if m.Running() {
		t.Error("done 후에는 실행 중이 아니어야 함")
	}
	_, done2, err := m.Begin(context.Background())
	if err != nil {
		t.Fatalf("끝난 뒤 새 작업 시작 실패: %v", err)
	}
	done()
	if !m.Running() {
		t.Error("이전 done이 새 작업을 지움")
	}
	done2()
}

func TestETA(t *testing.T) {
	base := time.Now()
	now := base
	e := &ETA{start: base, now: func() time.Time { return now }}
	if e.Remaining(0.5) != -1 {
		t.Error("2초 전에는 -1이어야 함")
	}
	now = base.Add(10 * time.Second)
	if got := e.Remaining(0.25); got != 30 {
		t.Errorf("Remaining(0.25) = %v, want 30", got)
	}
	if got := e.Remaining(1); got != 0 {
		t.Errorf("Remaining(1) = %v, want 0", got)
	}
}

func TestCancelAndWait(t *testing.T) {
	var m Manager
	if !m.CancelAndWait(time.Millisecond) {
		t.Fatal("작업이 없으면 바로 true")
	}
	ctx, done, _ := m.Begin(context.Background())
	go func() {
		<-ctx.Done()
		time.Sleep(20 * time.Millisecond)
		done()
	}()
	if !m.CancelAndWait(time.Second) {
		t.Fatal("정리가 끝날 때까지 기다려야 함")
	}
	if m.Running() {
		t.Error("기다린 뒤에는 실행 중이 아니어야 함")
	}
	_, done2, _ := m.Begin(context.Background())
	if m.CancelAndWait(10 * time.Millisecond) {
		t.Error("done을 안 부르면 시간 초과여야 함")
	}
	done2()
}
