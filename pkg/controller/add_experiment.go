package controller

import (
	"github.com/gramLabs/redsky/pkg/controller/experiment"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, experiment.Add)
}
