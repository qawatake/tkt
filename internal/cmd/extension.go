package cmd

import (
	"fmt"

	"github.com/qawatake/tkt/internal/extension"
	"github.com/spf13/cobra"
)

var extensionCmd = &cobra.Command{
	Use:   "extension",
	Short: "Manage tkt extensions",
	Long:  `Manage tkt extensions. Extensions are executables named tkt-* in your PATH.`,
}

var extensionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed extensions",
	Long:  `List all tkt extensions available in your PATH.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		manager := extension.NewManager()
		extensions, err := manager.FindExtensions()
		if err != nil {
			return fmt.Errorf("failed to find extensions: %v", err)
		}

		if len(extensions) == 0 {
			fmt.Println("No extensions found.")
			fmt.Println("Extensions are executables named 'tkt-*' in your PATH.")
			return nil
		}

		fmt.Printf("Found %d extension(s):\n", len(extensions))
		for _, ext := range extensions {
			fmt.Printf("  %s\t%s\n", ext.Name, ext.Path)
		}

		fmt.Println()
		fmt.Println("Usage: tkt <extension-name> [args...]")
		return nil
	},
}

func init() {
	extensionCmd.AddCommand(extensionListCmd)
	rootCmd.AddCommand(extensionCmd)
}
