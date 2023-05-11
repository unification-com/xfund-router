package server

import (
	"github.com/spf13/viper"
	"go-ooo/config"
)

const ServerContextKey = "server.context"

type Context struct {
	Viper  *viper.Viper
	Config *config.Config
}

func NewDefaultContext() *Context {
	return NewContext(
		viper.New(),
		config.DefaultConfig(),
	)
}

func NewContext(v *viper.Viper, config *config.Config) *Context {
	return &Context{v, config}
}
