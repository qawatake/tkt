package cmd

import (
	"os"
	"strings"

	"github.com/qawatake/tkt/internal/extension"
	"github.com/qawatake/tkt/internal/verbose"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tkt",
	Short: "JIRAチケットローカル同期CLI",
	Long:  `tktはJIRAチケットをローカルで編集し、それをリモートと同期するCLIツールです。`,
}

// Execute executes the root command.
func Execute() error {
	// Parse arguments to find the actual command after flags
	args := os.Args[1:]
	commandIndex := -1

	// Skip flags to find the actual command
	for i, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			commandIndex = i
			break
		}
		// Skip flag values - simple approach for common cases
		if arg == "-v" || arg == "--verbose" {
			continue
		}
		if strings.Contains(arg, "=") {
			// Flag with value attached (--flag=value)
			continue
		}
	}

	// If we found a potential command
	if commandIndex >= 0 && commandIndex < len(args) {
		subCmd := args[commandIndex]

		// Check if it's a known subcommand
		cmd, _, err := rootCmd.Find([]string{subCmd})
		if err == nil && cmd != rootCmd {
			// It's a known subcommand, execute normally
			return rootCmd.Execute()
		}

		// Try to execute as extension
		extManager := extension.NewManager()
		// Pass all args to the extension
		if err := extManager.Execute(subCmd, os.Args[1:]); err == nil {
			return nil
		}
	}

	// Default behavior
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose.Enabled, "verbose", "v", false, "enable verbose output")
}
