package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	"github.com/redskyops/k8s-experiment/pkg/api"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	"k8s.io/client-go/util/jsonpath"
	"sigs.k8s.io/yaml"
)

const (
	setLong    = `Modify the Red Sky Ops configuration file`
	setExample = `Names are: address, oauth2.token, oauth2.token_url, oauth2.client_id, oauth2.client_secret

# Set the remote server address
redskyctl config set address http://example.carbonrelay.io`
)

func NewSetCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewConfigOptions(ioStreams)
	o.Run = o.runSet
	o.Source = make(map[string]string)

	cmd := &cobra.Command{
		Use:     "set NAME [VALUE]",
		Short:   "Modify the configuration file",
		Long:    setLong,
		Example: setExample,
		Args:    cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, captureArgs(args, o)))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func captureArgs(args []string, o *ConfigOptions) []string {
	if len(args) > 1 {
		o.Source[args[0]] = args[1]
	} else if len(args) > 0 {
		o.Source[args[0]] = ""
	}
	return args
}

func (o *ConfigOptions) runSet() error {
	for k, v := range o.Source {
		// If this is an attempt to set an OAuth setting to a non-empty value, make sure we have someplace to put it
		if strings.HasPrefix(k, "oauth2.") && v != "" && o.Config.OAuth2 == nil {
			o.Config.OAuth2 = &api.OAuth2{}
		}

		// Evaluate the JSON path expression and set the result
		jp := jsonpath.New("config")
		if err := jp.Parse(fmt.Sprintf("{.%s}", k)); err != nil {
			return err
		}
		fullResults, err := jp.FindResults(o.Config)
		if err != nil {
			return err
		}
		if len(fullResults) == 1 && len(fullResults[0]) == 1 {
			fullResults[0][0].Set(reflect.ValueOf(v))
		} else {
			return fmt.Errorf("%s could not be set", k)
		}
	}

	// If there is nothing left in the configuration, remove the file
	if (api.Config{}) == *o.Config {
		err := os.Remove(o.ConfigFile)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	// Write the file to disk
	output, err := yaml.Marshal(o.Config)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(o.ConfigFile, output, 0644)
	return err
}
