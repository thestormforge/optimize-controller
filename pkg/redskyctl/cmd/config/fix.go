package config

import (
	"io/ioutil"
	"net/url"
	"path"
	"strings"

	client "github.com/redskyops/k8s-experiment/pkg/api"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

const (
	fixLong    = `Fix configurations to make them canonical, for example, upgrading from older versions`
	fixExample = ``
)

func NewFixCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewConfigOptions(ioStreams)
	o.Run = o.runFix

	cmd := &cobra.Command{
		Use:     "fix",
		Short:   "Fix configurations",
		Long:    fixLong,
		Example: fixExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *ConfigOptions) runFix() error {
	// Fix the configuration
	if err := FixConfig(o.Config); err != nil {
		return err
	}

	// Write it back to disk
	output, err := yaml.Marshal(o.Config)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(o.ConfigFile, output, 0644)
	return err
}

func FixConfig(config *client.Config) error {
	if config == nil {
		return nil
	}

	// Make sure the address parses
	address, err := url.Parse(config.Address)
	if err != nil {
		return err
	}

	// Remove old "/api" path component
	if removeAPI(address) {
		config.Address = address.String()
	}

	// Zero out default values
	if config.OAuth2 != nil && isDefaultTokenURL(address, config.OAuth2.TokenURL) {
		config.OAuth2.TokenURL = ""
	}

	return nil
}

// Removes 1.0.x "/api" suffix from the address
func removeAPI(address *url.URL) bool {
	if dir, file := path.Split(address.Path); file == "api" || strings.HasSuffix(dir, "/api/") {
		address.Path = strings.TrimRight(strings.TrimSuffix(dir, "/api/"), "/")
		return true
	}

	return false
}

// Checks to see if the token URL is a default value (implicit or explicit)
func isDefaultTokenURL(address *url.URL, tokenURL string) bool {
	if tokenURL == "../auth/token/" {
		return true
	}

	rel, _ := url.Parse("../auth/token/")
	if tokenURL == address.ResolveReference(rel).String() {
		return true
	}

	return false
}
