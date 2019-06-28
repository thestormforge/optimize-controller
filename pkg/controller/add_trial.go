package controller

import (
	"github.com/gramLabs/redsky/pkg/controller/trial"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, trial.Add)
}
