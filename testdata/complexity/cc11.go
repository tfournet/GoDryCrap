package thresholds

// CC11 adds a select clause inside a function literal, which counts toward
// the enclosing function: complexity 11.
func CC11(a, b int, ch chan int) int {
	n := 0
	if a > 0 {
		n++
	}
	if b > 0 {
		n++
	}
	for i := 0; i < a; i++ {
		n++
	}
	for range b {
		n++
	}
	if a > 0 && b > 0 {
		n++
	}
	if a < 0 || b < 0 {
		n++
	}
	switch a {
	case 1:
		n++
	}
	recv := func() {
		select {
		case v := <-ch:
			n += v
		}
	}
	recv()
	return n
}
