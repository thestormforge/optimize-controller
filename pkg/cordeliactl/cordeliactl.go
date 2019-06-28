package cordeliactl

import (
	"fmt"
	"os"

	"github.com/gramLabs/cordelia/pkg/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "cordeliactl",
}

func init() {
	rootCmd.Run = rootCmd.HelpFunc()
	rootCmd.Version = version.GetVersion()
	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newResetCommand())

	// TODO Add additional commands to the client
	// create experiment [--remote-only]


}

// Execute runs the application
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
