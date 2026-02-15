package electrum

import (
	reflect "reflect"
	"sync"
)

type Atomic[T any] struct {
	val T
	mu  sync.RWMutex
}

func MakeAtomic[T any](val T) *Atomic[T] {
	return &Atomic[T]{
		val: val,
	}
}

func (a *Atomic[T]) Get(f func(val T) (any, bool)) (any, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return f(a.val)
}

func (a *Atomic[T]) Do(f func(val T) error) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return f(a.val)
}

func (a *Atomic[T]) Change(f func(val T) (T, error)) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	val, err := f(a.val)
	if err != nil {
		return err
	}

	a.val = val
	return nil
}

func (a *Atomic[T]) IsNilOrZero() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	v := reflect.ValueOf(a.val)
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface || v.Kind() == reflect.Slice || v.Kind() == reflect.Map || v.Kind() == reflect.Chan || v.Kind() == reflect.Func {
		return v.IsNil()
	}

	return false
}

func (a *Atomic[T]) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.val = *new(T)
}

func Get[T any, R any](a *Atomic[T], f func(val T) (R, bool)) (R, bool) {
	embed := func(val T) (any, bool) {
		return f(val)
	}

	result, ok := a.Get(embed)
	if !ok {
		return *new(R), false
	}

	if val, ok := result.(R); ok {
		return val, true
	}

	return *new(R), false
}
