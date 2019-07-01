package util

import (
	"fmt"

	client "github.com/gramLabs/redsky/pkg/api"
	"github.com/spf13/pflag"
)

// Red Sky server specific configuration flags

type ServerFlags struct {
}

func NewServerFlags() *ServerFlags {
	return &ServerFlags{}
}

func (f *ServerFlags) AddFlags(flags *pflag.FlagSet) {}

func (f *ServerFlags) ToClientConfig() (*client.Config, error) {
	clientConfig, err := client.DefaultConfig()
	if err != nil {
		return nil, err
	}

	// TODO Apply overrides from the structure

	if clientConfig.Address == "" {
		// TODO What behavior do we want here? Error or default?
		return nil, fmt.Errorf("the server address is unspecified")
	}

	return clientConfig, nil
}
