package fmtx

import "fmt"

func Worker(text string, id int) string {
	return fmt.Sprintf("(worker %v) %v", id, text)
}
