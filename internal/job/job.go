// Package job은 한 번에 하나뿐인 취소 가능 작업을 관리한다.
package job

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrBusy = errors.New("다른 작업 실행 중")

type Manager struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	idle   chan struct{}
	seq    uint64
}

// 실행 중이면 ErrBusy. done은 자기 작업일 때만 자리를 비우고 idle을 닫는다.
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

func (m *Manager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
}

// 취소한 뒤 idle이 닫히길 timeout까지 기다린다. 제때 끝나면 true
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

func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cancel != nil
}

type ETA struct {
	start time.Time
	now   func() time.Time
}

func NewETA() *ETA { return &ETA{start: time.Now(), now: time.Now} }

// 경과 × 남은 비율 / 진행 비율. 2% 또는 2초 전에는 -1
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

type Throttle struct {
	interval time.Duration
	last     time.Time
	lastFrac float64
}

func NewThrottle(interval time.Duration) *Throttle {
	return &Throttle{interval: interval, lastFrac: -1}
}

// interval이 지났거나 처음으로 1.0에 닿으면 통과시킨다.
func (t *Throttle) Allow(frac float64) bool {
	now := time.Now()
	if frac >= 1 && t.lastFrac < 1 || now.Sub(t.last) >= t.interval {
		t.last = now
		t.lastFrac = frac
		return true
	}
	return false
}
