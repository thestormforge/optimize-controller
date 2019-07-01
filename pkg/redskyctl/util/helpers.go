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

func stringptr(val string) *string {
	return &val
}
