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
	setLong    = `TODO`
	setExample = `TODO`
)

func NewSetCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewConfigOptions(ioStreams)
	o.Run = o.runSet

	cmd := &cobra.Command{
		Use:     "set",
		Short:   "TODO",
		Long:    setLong,
		Example: setExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
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
