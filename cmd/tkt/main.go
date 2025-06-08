package main

import (
	"fmt"
	"os"

	"github.com/k1LoW/errors"
	"github.com/qawatake/tkt/internal/cmd"

	// サブコマンドを登録するため
	_ "github.com/qawatake/tkt/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		if st := errors.StackTraces(err); len(st) > 0 {
			fmt.Fprintf(os.Stderr, "Stack trace:\n%s\n", st)
		}
		os.Exit(1)
	}
}
