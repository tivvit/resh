package cmd

import (
	"fmt"

	"github.com/curusarn/resh/cmd/control/status"
	"github.com/spf13/cobra"
)

var exitCode status.Code

var rootCmd = &cobra.Command{
	Use:   "reshctl",
	Short: "Reshctl (RESH control) - enables you to enable/disable features and more.",
	Long:  `Enables you to enable/disable RESH bindings for arrows and C-R.`,
}

// Execute reshctl
func Execute() status.Code {
	rootCmd.AddCommand(disableCmd)
	disableCmd.AddCommand(disableArrowKeyBindingsCmd)
	disableCmd.AddCommand(disableArrowKeyBindingsGlobalCmd)

	rootCmd.AddCommand(enableCmd)
	enableCmd.AddCommand(enableArrowKeyBindingsCmd)
	enableCmd.AddCommand(enableArrowKeyBindingsGlobalCmd)

	rootCmd.AddCommand(completionCmd)
	completionCmd.AddCommand(completionBashCmd)
	completionCmd.AddCommand(completionZshCmd)

	rootCmd.AddCommand(debugCmd)
	debugCmd.AddCommand(debugReloadCmd)
	debugCmd.AddCommand(debugOutputCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		return status.Fail
	}
	return exitCode
}
