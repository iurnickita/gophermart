package config

import (
	"flag"
	"os"

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
	cfg := Config{}

	flag.StringVar(&cfg.Handler.ServerAddr, "a", "localhost:8080", "address of HTTP server")
	flag.StringVar(&cfg.Store.DBDsn, "d", "", "database dsn")
	flag.StringVar(&cfg.Service.AccrualAddr, "r", "", "accrual system address")
	flag.StringVar(&cfg.Logger.LogLevel, "l", "info", "log level")

	if envrun := os.Getenv("RUN_ADDRESS"); envrun != "" {
		cfg.Handler.ServerAddr = envrun
	}
	if envdsn := os.Getenv("DATABASE_URI"); envdsn != "" {
		cfg.Store.DBDsn = envdsn
	}
	if envaccr := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envaccr != "" {
		cfg.Service.AccrualAddr = envaccr
	}

	return cfg
}
