// Package job은 한 번에 하나뿐인 취소 가능 작업을 관리한다.
package job

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrBusy는 다른 작업이 실행 중일 때 반환된다.
var ErrBusy = errors.New("다른 작업 실행 중")

// Manager는 동시에 한 작업만 허용한다.
type Manager struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	idle   chan struct{}
	seq    uint64
}

// Begin은 새 작업을 시작한다. done은 꼭 불러야 한다.
func (m *Manager) Begin(parent context.Context) (ctx context.Context, done func(), err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return nil, nil, ErrBusy
	}
	ctx, cancel := context.WithCancel(parent)
	idle := make(chan struct{})
	m.seq++
	id := m.seq
	m.cancel, m.idle = cancel, idle
	return ctx, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		cancel()
		if m.seq == id && m.cancel != nil {
			m.cancel, m.idle = nil, nil
			close(idle)
		}
	}, nil
}

// Cancel은 실행 중인 작업을 취소한다.
func (m *Manager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
}

// CancelAndWait는 취소 후 정리가 끝나길 기다린다.
func (m *Manager) CancelAndWait(timeout time.Duration) bool {
	m.mu.Lock()
	idle := m.idle
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()
	if idle == nil {
		return true
	}
	select {
	case <-idle:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Running은 작업이 실행 중인지 알려준다.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cancel != nil
}

// ETA는 진행 비율로 남은 시간을 추정한다.
type ETA struct {
	start time.Time
	now   func() time.Time
}

func NewETA() *ETA { return &ETA{start: time.Now(), now: time.Now} }

// Remaining은 남은 초. 추정이 이르면 -1.
func (e *ETA) Remaining(frac float64) float64 {
	elapsed := e.now().Sub(e.start).Seconds()
	if frac < 0.02 || elapsed < 2 {
		return -1
	}
	if frac >= 1 {
		return 0
	}
	return elapsed * (1 - frac) / frac
}

// Throttle은 interval마다 한 번만 통과시킨다. 1.0은 항상.
type Throttle struct {
	interval time.Duration
	last     time.Time
	lastFrac float64
}

func NewThrottle(interval time.Duration) *Throttle {
	return &Throttle{interval: interval, lastFrac: -1}
}

func (t *Throttle) Allow(frac float64) bool {
	now := time.Now()
	if frac >= 1 && t.lastFrac < 1 || now.Sub(t.last) >= t.interval {
		t.last = now
		t.lastFrac = frac
		return true
	}
	return false
}
