package edge

import (
	"errors"
	"fmt"
)

var Hook = func(x int) int {
	if x > 0 {
		return x
	}
	return -x
}

func Empty() {}

func EmptyMultiline() {
}

func ElseChain(x int) string {
	if x < 0 {
		return "neg"
	} else if x == 0 {
		return "zero"
	} else /* comment */ if x < 10 {
		return "small"
	} else {
		return "big"
	}
}

func EmptyBranches(x int) {
	if x > 0 {
	} else {
	}
}

func Labels(n int) int {
	total := 0
outer:
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if j > i {
				continue outer
			}
			total++
		}
	}
	i := 0
loop:
	i++
	if i < n {
		goto loop
	}
	return total
}

func Panics(err error) {
	if err != nil {
		panic(err)
	}
	fmt.Println("ok")
	panic("unreachable")
}

func Literals(xs []int) int {
	sum := 0
	each := func(f func(int)) {
		for _, x := range xs {
			f(x)
		}
	}
	each(func(x int) { sum += x })
	if f := func() bool { return sum > 0 }; f() {
		sum++
	}
	for i := func() int { return 0 }(); i < 2; i++ {
		sum += i
	}
	switch v := func() int { return sum }(); {
	case v > 10:
		sum = 10
	}
	defer func() {
		sum = 0
	}()
	go func() {}()
	return sum
}

func Switches(x any, ch chan int) string {
	switch {
	}
	switch x {
	}
	switch x.(type) {
	}
	select {}
	switch v := x.(type) {
	case int:
		return fmt.Sprint(v)
	case string, error:
	default:
		return "other"
	}
	select {
	case v := <-ch:
		return fmt.Sprint(v)
	case ch <- 1:
	default:
	}
	return ""
}

func Gaps(x int) int {
	a := x + 1

	// a comment line

	b := a * 2
	/* block
	   comment */
	c := b - 1
	const (
		k1 = 1
		k2 = 2
	)
	s := `multi
line
string`
	_ = s
	return c + k1 + k2
}

func Blocks(x int) int {
	{
		x++
	}
	{
	}
	return x
}

func Errors(err error) bool {
	var target *myErr
	return errors.As(err, &target) && target != nil
}

type myErr struct{}

func (*myErr) Error() string { return "e" }

func _() { fmt.Println("never") }

func Ranges(m map[string]int) (n int) {
	for range m {
		n++
	}
	for k, v := range m {
		if k == "" {
			break
		}
		n += v
	}
	return
}
