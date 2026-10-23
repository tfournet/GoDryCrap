package game

import "testing"

func TestPlace(t *testing.T) {
	ready, ok = true, true
	(&Board{}).Place()
}
