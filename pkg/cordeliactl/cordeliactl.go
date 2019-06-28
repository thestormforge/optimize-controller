package cordeliactl

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "cordeliactl",
}

func init() {
	rootCmd.Run = rootCmd.HelpFunc()
	rootCmd.Version = Version()
	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newResetCommand())

	// TODO Add additional commands to the client
	// create experiment [--remote-only]

}

// Version returns the current version of the application
func Version() string {
	return "v1.0.0-alpha.1"
}

// Execute runs the application
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
