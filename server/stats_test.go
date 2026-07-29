// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatsIncrementAndSwap(t *testing.T) {
	s := &stats{}

	s.incrementAndroidSend()
	s.incrementAndroidSend()
	s.incrementAppleSend()
	s.incrementAck()
	s.incrementAck()
	s.incrementAck()

	android, apple, acks := s.swap()
	assert.Equal(t, int64(2), android)
	assert.Equal(t, int64(1), apple)
	assert.Equal(t, int64(3), acks)

	// swap must reset the counters back to zero.
	android, apple, acks = s.swap()
	assert.Equal(t, int64(0), android)
	assert.Equal(t, int64(0), apple)
	assert.Equal(t, int64(0), acks)
}

func TestStatsNilReceiverIsSafe(t *testing.T) {
	var s *stats

	assert.NotPanics(t, func() {
		s.incrementAndroidSend()
		s.incrementAppleSend()
		s.incrementAck()
	})
}

func TestStatsConcurrentIncrement(t *testing.T) {
	s := &stats{}

	const goroutines = 50
	const perGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range perGoroutine {
				s.incrementAndroidSend()
				s.incrementAppleSend()
				s.incrementAck()
			}
		}()
	}
	wg.Wait()

	want := int64(goroutines * perGoroutine)
	android, apple, acks := s.swap()
	assert.Equal(t, want, android)
	assert.Equal(t, want, apple)
	assert.Equal(t, want, acks)
}
