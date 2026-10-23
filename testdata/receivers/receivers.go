package receivers

type Value struct{ n int }

type Stack[T any] struct{ items []T }

type Pair[K comparable, V any] struct {
	k K
	v V
}

func (v Value) Get() int { return v.n }

func (v *Value) Set(n int) {
	if n < 0 {
		n = 0
	}
	v.n = n
}

func (s *Stack[T]) Push(item T) { s.items = append(s.items, item) }

func (p Pair[K, V]) Key() K { return p.k }

func (Value) Kind() string { return "value" }

func init() {}

func init() {}

func declared() int
