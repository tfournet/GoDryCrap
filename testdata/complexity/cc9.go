package thresholds

// CC9 has eight decisions (if, if, for, range, if and &&, if and ||):
// complexity 9, the warn level.
func CC9(a, b int) int {
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
	return n
}
