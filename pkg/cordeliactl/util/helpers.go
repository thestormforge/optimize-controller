package util

import (
	"fmt"
	"os"
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

// TODO Should we combine `HomeDir` with `filepath.Join`? If so we need to add an `error` to the return since we can't check for ""

// HomeDir returns the user's home directory.
func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE")
}
