package main

import "sync/atomic"

const (
	statQueryCount = iota
	statQueryFailed
	statQuerySkipped
	statConnCount
	statConnOpen
	statConnFailed
)

type stats map[int]*int32

func newStats() stats {
	st := stats{}
	for i := statQueryCount; i <= statConnFailed; i++ {
		st[i] = new(int32)
	}
	return st
}

func (s stats) get(counter int) int32 {
	if v, ok := s[counter]; ok {
		return *v
	}
	return 0
}

func (s stats) inc(counter int) {
	s.add(counter, 1)
}

func (s stats) dec(counter int) {
	s.add(counter, -1)
}

func (s stats) add(counter int, delta int32) {
	atomic.AddInt32(s[counter], delta)
}
