package main

import (
	"fmt"
)

func debugf(format string, args ...interface{}) {
	if !*debug {
		return
	}
	fmt.Printf(format, args...)
}




