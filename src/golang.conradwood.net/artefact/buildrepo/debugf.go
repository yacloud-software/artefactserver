package buildrepo

import (
	"fmt"
)

func debugf(format string, args ...interface{}) {
	if !*debug {
		return
	}
	s := fmt.Sprintf(format, args...)
	fmt.Print("[buildrepo] " + s)
}


