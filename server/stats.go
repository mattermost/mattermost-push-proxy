// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"sync/atomic"
	"time"
)

// statsReportInterval controls how often aggregated throughput is logged.
// Per-notification events are logged at Debug; this keeps a low-volume
// operational heartbeat at Info.
const statsReportInterval = time.Minute

// stats holds lightweight atomic counters for notification throughput. It is
// kept independent of the Prometheus metrics so the heartbeat works even when
// metrics are disabled. All methods are safe to call on a nil receiver.
type stats struct {
	androidSends atomic.Int64
	appleSends   atomic.Int64
	acks         atomic.Int64
}

func (s *stats) incrementAndroidSend() {
	if s == nil {
		return
	}
	s.androidSends.Add(1)
}

func (s *stats) incrementAppleSend() {
	if s == nil {
		return
	}
	s.appleSends.Add(1)
}

func (s *stats) incrementAck() {
	if s == nil {
		return
	}
	s.acks.Add(1)
}

// swap atomically reads and resets the counters, returning the totals
// accumulated since the previous call.
func (s *stats) swap() (android, apple, acks int64) {
	if s == nil {
		return 0, 0, 0
	}
	return s.androidSends.Swap(0), s.appleSends.Swap(0), s.acks.Swap(0)
}
