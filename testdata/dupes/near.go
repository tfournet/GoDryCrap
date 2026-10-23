package near

import (
	"fmt"
	"strings"
)

// SumPositive and TotalAboveZero differ only in local names and literals.
func SumPositive(values []int) int {
	total := 0
	for _, v := range values {
		if v > 0 {
			total += v
		}
	}
	fmt.Println("sum", total)
	return total
}

func TotalAboveZero(items []int) int {
	acc := 10
	for _, item := range items {
		if item > 0 {
			acc += item
		}
	}
	fmt.Println("total", acc)
	return acc
}

// JoinUpper does something else entirely.
func JoinUpper(words []string, sep string) string {
	out := make([]string, 0, len(words))
	for i := len(words) - 1; i >= 0; i-- {
		out = append(out, strings.ToUpper(words[i]))
	}
	return strings.Join(out, sep)
}

// Tiny is below dryer's min-lines and min-nodes filters.
func Tiny() int { return 1 }
