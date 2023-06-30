package metrics

import (
	"sync"
)

// TssKeysignMetricMgr is a struct to manage tss keysign metric in memory
type TssKeysignMetricMgr struct {
	lock          *sync.Mutex
	keysignMetric map[string]int64
}

// NewTssKeysignMetricMgr create a new instance of TssKeysignMetricMgr
func NewTssKeysignMetricMgr() *TssKeysignMetricMgr {
	return &TssKeysignMetricMgr{
		lock:          &sync.Mutex{},
		keysignMetric: make(map[string]int64),
	}
}

// SetTssKeysignMetric save the tss keysign time metric against the given hash
func (m *TssKeysignMetricMgr) SetTssKeysignMetric(hash string, elapseInMs int64) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.keysignMetric[hash] = elapseInMs
}

// GetTssKeysignMetric get the metric of the given hash , and delete it after
func (m *TssKeysignMetricMgr) GetTssKeysignMetric(hash string) int64 {
	m.lock.Lock()
	defer m.lock.Unlock()
	elapse, ok := m.keysignMetric[hash]
	if ok {
		delete(m.keysignMetric, hash)
	}
	return elapse
}
