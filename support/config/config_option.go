package config

import (
	"fmt"
	"go/types"
	stdLog "log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/support/strutils"
)

// ConfigOptions is a group of ConfigOptions that can be for convenience
// initialized and set at the same time.
type ConfigOptions []*ConfigOption

// Init calls Init on each ConfigOption passing on the cobra.Command.
func (cos ConfigOptions) Init(cmd *cobra.Command) error {
	for _, co := range cos {
		if err := co.Init(cmd); err != nil {
			return err
		}
		co.SetDeprecated(cmd)
	}
	return nil
}

// Require calls Require on each ConfigOption.
func (cos ConfigOptions) Require() {
	for _, co := range cos {
		co.Require()
	}
}

// RequireE is like Require, but returns the error instead of Fatal
func (cos ConfigOptions) RequireE() error {
	for _, co := range cos {
		if err := co.RequireE(); err != nil {
			return err
		}
	}
	return nil
}

// SetValues calls SetValue on each ConfigOption.
func (cos ConfigOptions) SetValues() error {
	for _, co := range cos {
		if err := co.SetValue(); err != nil {
			return err
		}
	}
	return nil
}

// GetCommandLineFlagsPassedByUser returns a list of command-line flags that were passed by the user when running Horizon.
func (cos ConfigOptions) GetCommandLineFlagsPassedByUser() []string {
	var flagsPassedByUser []string
	for _, co := range cos {
		if co.flag.Changed {
			flagsPassedByUser = append(flagsPassedByUser, co.flag.Name)
		}
	}
	return flagsPassedByUser
}

// ConfigOption is a complete description of the configuration of a command line option
type ConfigOption struct {
	Name           string                    // e.g. "db-url"
	EnvVar         string                    // e.g. "DATABASE_URL". Defaults to uppercase/underscore representation of name
	OptType        types.BasicKind           // The type of this option, e.g. types.Bool
	FlagDefault    interface{}               // A default if no option is provided. Omit or set to `nil` if no default
	Required       bool                      // Whether this option must be set for Horizon to run
	Usage          string                    // Help text
	CustomSetValue func(*ConfigOption) error // Optional function for custom validation/transformation
	ConfigKey      interface{}               // Pointer to the final key in the linked Config struct
	flag           *pflag.Flag               // The persistent flag that the config option is attached to
	Hidden         bool                      // Indicates whether to hide the flag from --help output
	UsedInCommands []string                  // the list of sub-comands this flag is relevant in
}

// Init handles initialisation steps, including configuring and binding the env variable name.
func (co *ConfigOption) Init(cmd *cobra.Command) error {
	// Bind the command line and environment variable name
	// Unless overriden, default to a transform like tls-key -> TLS_KEY
	if co.EnvVar == "" {
		co.EnvVar = strutils.KebabToConstantCase(co.Name)
	}
	// Initialise and bind the persistent flags
	return co.setFlag(cmd)
}

// SetDeprecated Hides the deprecated flag from --help output
func (co *ConfigOption) SetDeprecated(cmd *cobra.Command) {
	if co.Hidden {
		co.ToggleHidden(true)
	}
}

// ToggleHidden sets the hidden attibute on the persistent flag associated to
// this config option
func (co *ConfigOption) ToggleHidden(hidden bool) {
	co.flag.Hidden = hidden
}

// Bind binds the config option to viper.
func (co *ConfigOption) Bind() {
	viper.BindPFlag(co.Name, co.flag)
	viper.BindEnv(co.Name, co.EnvVar)
}

// Require checks that a required string configuration option is not empty, raising a user error if it is.
func (co *ConfigOption) Require() {
	if err := co.RequireE(); err != nil {
		stdLog.Fatal(err.Error())
	}
}

// RequireE is like Require, but returns the error instead of Fatal
func (co *ConfigOption) RequireE() error {
	co.Bind()
	if co.Required && viper.GetString(co.Name) == "" {
		return fmt.Errorf("Invalid config: %s is blank. Please specify --%s on the command line or set the %s environment variable.", co.Name, co.Name, co.EnvVar)
	}
	return nil
}

// SetValue sets a value in the global config, using a custom function, if one was provided.
func (co *ConfigOption) SetValue() error {
	co.Bind()

	// Use a custom setting function, if one is provided
	if co.CustomSetValue != nil {
		if err := co.CustomSetValue(co); err != nil {
			return err
		}
		// Otherwise, just set the provided arg directly
	} else if co.ConfigKey != nil {
		co.setSimpleValue()
	}
	return nil
}

// UsageText returns the string to use for the usage text of the option. The
// string returned will be the Usage defined on the ConfigOption, along with
// the environment variable.
func (co *ConfigOption) UsageText() string {
	return fmt.Sprintf("%s (%s)", co.Usage, co.EnvVar)
}

// setSimpleValue sets the value of a ConfigOption's configKey, based on the ConfigOption's default type.
func (co *ConfigOption) setSimpleValue() {
	if co.ConfigKey != nil {
		switch co.OptType {
		case types.String:
			*(co.ConfigKey.(*string)) = viper.GetString(co.Name)
		case types.Int:
			*(co.ConfigKey.(*int)) = viper.GetInt(co.Name)
		case types.Bool:
			*(co.ConfigKey.(*bool)) = viper.GetBool(co.Name)
		case types.Uint:
			*(co.ConfigKey.(*uint)) = uint(viper.GetInt(co.Name))
		case types.Uint32:
			*(co.ConfigKey.(*uint32)) = uint32(viper.GetInt(co.Name))
		case types.Float64:
			*(co.ConfigKey.(*float64)) = float64(viper.GetFloat64(co.Name))
		}
	}
}

// setFlag sets the correct pFlag type, based on the ConfigOption's default type.
func (co *ConfigOption) setFlag(cmd *cobra.Command) error {
	switch co.OptType {
	case types.String:
		// Set an empty string if no default was provided, since some value is always required for pflags
		if co.FlagDefault == nil {
			co.FlagDefault = ""
		}
		cmd.PersistentFlags().String(co.Name, co.FlagDefault.(string), co.UsageText())
	case types.Int:
		cmd.PersistentFlags().Int(co.Name, co.FlagDefault.(int), co.UsageText())
	case types.Bool:
		cmd.PersistentFlags().Bool(co.Name, co.FlagDefault.(bool), co.UsageText())
	case types.Uint:
		cmd.PersistentFlags().Uint(co.Name, co.FlagDefault.(uint), co.UsageText())
	case types.Uint32:
		cmd.PersistentFlags().Uint32(co.Name, co.FlagDefault.(uint32), co.UsageText())
	case types.Float64:
		cmd.PersistentFlags().Float64(co.Name, co.FlagDefault.(float64), co.UsageText())
	default:
		return errors.New("Unexpected OptType")
	}

	co.flag = cmd.PersistentFlags().Lookup(co.Name)

	return nil
}

// SetDuration converts a command line int to a duration, and stores it in the final config.
func SetDuration(co *ConfigOption) error {
	*(co.ConfigKey.(*time.Duration)) = time.Duration(viper.GetInt(co.Name)) * time.Second
	return nil
}

// SetDurationMinutes converts a command line minutes value to a duration, and stores it in the final config.
func SetDurationMinutes(co *ConfigOption) error {
	*(co.ConfigKey.(*time.Duration)) = time.Duration(viper.GetInt(co.Name)) * time.Minute
	return nil
}

// SetURL converts a command line string to a URL, and stores it in the final config.
func SetURL(co *ConfigOption) error {
	urlString := viper.GetString(co.Name)
	if urlString != "" {
		urlType, err := url.Parse(urlString)
		if err != nil {
			return fmt.Errorf("Unable to parse URL: %s/%v", urlString, err)
		}
		*(co.ConfigKey.(**url.URL)) = urlType
	}
	return nil
}

// SetOptionalUint converts a command line uint to a *uint where the nil
// value indicates the flag was not explicitly set
func SetOptionalUint(co *ConfigOption) error {
	key := co.ConfigKey.(**uint)
	if IsExplicitlySet(co) {
		*key = new(uint)
		**key = uint(viper.GetInt(co.Name))
	} else {
		*key = nil
	}
	return nil
}

func parseEnvVars(entries []string) map[string]bool {
	set := map[string]bool{}
	for _, entry := range entries {
		key := strings.Split(entry, "=")[0]
		set[key] = true
	}
	return set
}

// IsExplicitlySet returns true if and only if the given config option was set explicitly either
// via a command line argument or via an environment variable
func IsExplicitlySet(co *ConfigOption) bool {
	// co.flag.Changed is only set to true when the configuration is set via command line parameter.
	// In the case where a variable is configured via environment variable we need to check envVars.
	envVars := parseEnvVars(os.Environ())
	return co.flag.Changed || envVars[co.EnvVar]
}

// SetOptionalString converts a command line uint to a *string where the nil
// value indicates the flag was not explicitly set
func SetOptionalString(co *ConfigOption) error {
	key := co.ConfigKey.(**string)
	if IsExplicitlySet(co) {
		*key = new(string)
		**key = viper.GetString(co.Name)
	} else {
		*key = nil
	}
	return nil
}
