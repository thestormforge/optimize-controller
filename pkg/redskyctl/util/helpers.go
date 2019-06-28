package util

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// CheckErr ensures the supplied error is reported and handled correctly. Errors may be reported and/or may cause the process to exit.
func CheckErr(err error) {
	if err == nil {
		return
	}

	// This error handling leaves a lot to be desired...
	fmt.Fprintf(os.Stderr, "Failed: %s\n", err.Error())
	os.Exit(1)
}

func KubeConfig(cmd *cobra.Command, kubeconfig *string) {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = os.Getenv("USERPROFILE")
	}
	if homeDir != "" {
		*kubeconfig = filepath.Join(homeDir, ".kube", "config")
	}

	cmd.Flags().StringVar(kubeconfig, "kubeconfig", *kubeconfig, "absolute path to the kubeconfig file")
	if *kubeconfig == "" {
		_ = cmd.MarkFlagRequired("kubeconfig")
	}
}
