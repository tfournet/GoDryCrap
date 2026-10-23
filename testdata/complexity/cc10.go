package thresholds

// CC10 adds a switch with one case: complexity 10, the fail level.
func CC10(a, b int) int {
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
	return n
}
