package metrics

import (
	"sync"
	"time"
)

type TimeKeeper struct {
	mu                     sync.Mutex
	previousVoteByFp       map[string]*time.Time
	previousRandomnessByFp map[string]*time.Time
}

func NewTimeKeeper() *TimeKeeper {
	return &TimeKeeper{
		mu: sync.Mutex{},
	}
}

func (mt *TimeKeeper) RecordVoteTime(fpBtcPkHex string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	now := time.Now()

	if mt.previousVoteByFp == nil {
		mt.previousVoteByFp = make(map[string]*time.Time)
	}
	mt.previousVoteByFp[fpBtcPkHex] = &now
}

func (mt *TimeKeeper) RecordRandomnessTime(fpBtcPkHex string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	now := time.Now()

	if mt.previousRandomnessByFp == nil {
		mt.previousRandomnessByFp = make(map[string]*time.Time)
	}
	mt.previousRandomnessByFp[fpBtcPkHex] = &now
}
