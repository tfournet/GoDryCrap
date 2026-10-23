package sample

import "testing"

func TestSome(t *testing.T) {
	Branchy(5, nil)
	Branchy(20, nil)
	(&Widget{}).Run()
}
