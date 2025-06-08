package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tkt",
	Short: "JIRAチケットローカル同期CLI",
	Long:  `tktはJIRAチケットをローカルで編集し、それをリモートと同期するCLIツールです。`,
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// ここでグローバルフラグなどを設定する場合は追加
}
