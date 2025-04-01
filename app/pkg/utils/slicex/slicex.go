package slicex

import "golang.org/x/exp/constraints"

func Sum[T constraints.Ordered](arr []T) T {
	var sum T
	for _, value := range arr {
		sum += value
	}
	return sum
}
