package service

import (
	"fmt"
	"sync"
)

type flightGroup struct {
	mu    sync.Mutex
	calls map[string]*flightCall
}

type flightCall struct {
	done chan struct{}
	val  any
	err  error
}

func newFlightGroup() *flightGroup {
	return &flightGroup{calls: map[string]*flightCall{}}
}

func (g *flightGroup) do(key string, fn func() (any, error)) (any, error) {
	g.mu.Lock()
	if call, ok := g.calls[key]; ok {
		g.mu.Unlock()
		<-call.done
		return call.val, call.err
	}
	call := &flightCall{done: make(chan struct{})}
	g.calls[key] = call
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		delete(g.calls, key)
		g.mu.Unlock()
		close(call.done)
	}()

	call.val, call.err = fn()
	return call.val, call.err
}

func flightDo[T any](group *flightGroup, key string, fn func() (T, error)) (T, error) {
	if group == nil {
		return fn()
	}

	value, err := group.do(key, func() (any, error) {
		return fn()
	})
	if value == nil {
		var zero T
		return zero, err
	}

	typed, ok := value.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("single-flight result type mismatch")
	}
	return typed, err
}
