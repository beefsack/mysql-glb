package main

import "sync"

type intLocker struct {
	m    *sync.Mutex
	intM map[int]*sync.Mutex
}

func newIntLocker() *intLocker {
	return &intLocker{
		m:    &sync.Mutex{},
		intM: map[int]*sync.Mutex{},
	}
}

func (il *intLocker) get(i int) *sync.Mutex {
	il.m.Lock()
	defer il.m.Unlock()
	intM, ok := il.intM[i]
	if !ok {
		intM = &sync.Mutex{}
		il.intM[i] = intM
	}
	return intM
}

func (il *intLocker) lock(i int) {
	il.get(i).Lock()
}

func (il *intLocker) unlock(i int) {
	il.get(i).Unlock()
}
