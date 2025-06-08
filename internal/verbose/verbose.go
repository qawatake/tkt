package verbose

import (
	"fmt"
)

var Enabled bool

func Printf(format string, args ...any) {
	if Enabled {
		fmt.Printf(format, args...)
	}
}

func Println(args ...any) {
	if Enabled {
		fmt.Println(args...)
	}
}

func Print(args ...any) {
	if Enabled {
		fmt.Print(args...)
	}
}
