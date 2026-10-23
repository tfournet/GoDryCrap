package syntax

import (
	"errors"
	"fmt"
	"sync"
)

type Pair[K comparable, V any] struct {
	Key   K `json:"key"`
	Value V
	sync.Mutex
}

type Shape interface {
	Area() float64
	fmt.Stringer
}

// Everything uses every statement and expression kind the normalizer maps.
func Everything[T any, U fmt.Stringer](in []T, m map[string]*Pair[string, int], ch chan int, fns ...func(int) error) (out []T, err error) {
	const limit, other = iota, 2
	var (
		x, y int = 1, 2
		z    [3]int
	)
	type local struct{ a, b int }
	type alias = local
	arr := [...]int{1, 2, 3}
	p := &Pair[string, int]{Key: "k", Value: 1}
	s := []alias{{1, 2}, {a: 3}}
	q := Pair[int, bool]{}
	_, _, _, _ = arr, p, s, q
	if v, ok := m["a"]; ok && v != nil || !ok {
		x++
	} else if x > y {
		y--
	} else {
		z[0] = -x
	}
	for i := 0; i < limit; i++ {
		continue
	}
	for x < other {
		break
	}
	for {
		goto done
	}
done:
	for k, v := range m {
		_ = k + v.Key
	}
	for range ch {
	}
	switch x {
	case 1, 2:
		fallthrough
	case 3:
	default:
	}
	switch {
	}
	var any_ any = in
	switch t := any_.(type) {
	case int, *string:
		_ = t
	case nil:
	default:
	}
	switch any_.(type) {
	}
	select {
	case v := <-ch:
		_ = v
	case ch <- 1:
	case <-ch:
	default:
	}
	go func() { ch <- x }()
	defer func(a, b int) {}(x, y)
	x <<= 2
	x &^= 1
	_ = in[1:2]
	_ = in[:1:2]
	_ = any_.(Shape)
	_ = *p
	_ = <-ch
	_ = (x + y) * 2
	_ = fmt.Sprintf("%v", z[:]...)
	_ = func(f func() (int, error)) chan<- int { return nil }
	_ = struct{}{}
	_ = interface{ ~int | string }(nil)
	_ = errors.New("x")
	_ = 1.5 + 'c'
	_ = true && false
	return nil, fmt.Errorf("%w", err)
}
