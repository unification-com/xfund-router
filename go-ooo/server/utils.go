package server

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go-ooo/config"
	"go-ooo/flags"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func InterceptConfigsPreRunHandler(cmd *cobra.Command) error {
	serverCtx := NewDefaultContext()

	executableName, err := os.Executable()
	if err != nil {
		return err
	}

	basename := path.Base(executableName)
	// Configure the viper instance
	serverCtx.Viper.BindPFlags(cmd.Flags())
	serverCtx.Viper.BindPFlags(cmd.PersistentFlags())
	serverCtx.Viper.SetEnvPrefix(basename)
	serverCtx.Viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	serverCtx.Viper.AutomaticEnv()

	// intercept configuration files, using both Viper instances separately
	config, err := interceptConfigs(serverCtx.Viper)
	if err != nil {
		return err
	}

	serverCtx.Config = config

	if err = bindFlags(basename, cmd, serverCtx.Viper); err != nil {
		return err
	}

	// Todo - use srvCtx for logger

	return SetCmdServerContext(cmd, serverCtx)

}

// GetServerContextFromCmd returns a Context from a command or an empty Context
// if it has not been set.
func GetServerContextFromCmd(cmd *cobra.Command) *Context {
	if v := cmd.Context().Value(ServerContextKey); v != nil {
		serverCtxPtr := v.(*Context)
		return serverCtxPtr
	}

	return NewDefaultContext()
}

// SetCmdServerContext sets a command's Context value to the provided argument.
func SetCmdServerContext(cmd *cobra.Command, serverCtx *Context) error {
	v := cmd.Context().Value(ServerContextKey)
	if v == nil {
		return errors.New("server context not set")
	}

	serverCtxPtr := v.(*Context)
	*serverCtxPtr = *serverCtx

	return nil
}

func bindFlags(basename string, cmd *cobra.Command, v *viper.Viper) (err error) {
	defer func() {
		recover()
	}()

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them, so bind them to their equivalent
		// keys with underscores, e.g. --favorite-color to STING_FAVORITE_COLOR
		err = v.BindEnv(f.Name, fmt.Sprintf("%s_%s", basename, strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))))
		if err != nil {
			panic(err)
		}

		err = v.BindPFlag(f.Name, f)
		if err != nil {
			panic(err)
		}

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			err = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
			if err != nil {
				panic(err)
			}
		}
	})

	return
}

func interceptConfigs(rootViper *viper.Viper) (*config.Config, error) {
	rootDir := rootViper.GetString(flags.FlagHome)
	tmCfgFile := filepath.Join(rootDir, "config.toml")
	conf := config.DefaultConfig()

	switch _, err := os.Stat(tmCfgFile); {
	case os.IsNotExist(err):
		return nil, fmt.Errorf("config file does not exist. Run init command")
	case err != nil:
		return nil, err
	default:
		rootViper.SetConfigType("toml")
		rootViper.SetConfigName("config")
		rootViper.AddConfigPath(rootDir)

		if err := rootViper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read in %s: %w", tmCfgFile, err)
		}
	}

	if err := rootViper.Unmarshal(conf); err != nil {
		return nil, err
	}

	return conf, nil
}
