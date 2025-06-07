package verbose

import (
	"fmt"
	"os"
)

var Enabled bool

func init() {
	// コマンドライン引数から-vフラグをチェック
	for _, arg := range os.Args {
		if arg == "-v" || arg == "--verbose" {
			Enabled = true
			break
		}
	}
}

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
