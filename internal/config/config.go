package config

import (
	handlerConfig "github.com/iurnickita/gophermart/internal/handler/config"
	loggerConfig "github.com/iurnickita/gophermart/internal/logger/config"
	serviceConfig "github.com/iurnickita/gophermart/internal/service/config"
	storeConfig "github.com/iurnickita/gophermart/internal/store/config"
)

type Config struct {
	Handler handlerConfig.Config
	Service serviceConfig.Config
	Store   storeConfig.Config
	Logger  loggerConfig.Config
}

func GetConfig() Config {
	return Config{}
}
