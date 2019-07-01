package util

import (
	"fmt"

	client "github.com/gramLabs/redsky/pkg/api"
	"github.com/spf13/pflag"
)

// Red Sky server specific configuration flags

const (
	flagAddress = "address"
)

type ServerFlags struct {
	Address *string
}

func NewServerFlags() *ServerFlags {
	return &ServerFlags{
		Address: stringptr(""),
	}
}

func (f *ServerFlags) AddFlags(flags *pflag.FlagSet) {
	if f.Address != nil {
		flags.StringVar(f.Address, flagAddress, *f.Address, "Absolute URL of the Red Sky API.")
	}
}

func (f *ServerFlags) ToClientConfig() (*client.Config, error) {
	clientConfig, err := client.DefaultConfig()
	if err != nil {
		return nil, err
	}

	if f.Address != nil && *f.Address != "" {
		clientConfig.Address = *f.Address
	}

	if clientConfig.Address == "" {
		// TODO What behavior do we want here? Error or default?
		return nil, fmt.Errorf("the server address is unspecified")
	}

	return clientConfig, nil
}
