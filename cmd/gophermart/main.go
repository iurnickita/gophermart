package main

import (
	"log"

	"github.com/iurnickita/gophermart/internal/auth"
	"github.com/iurnickita/gophermart/internal/config"
	"github.com/iurnickita/gophermart/internal/handler"
	"github.com/iurnickita/gophermart/internal/logger"
	"github.com/iurnickita/gophermart/internal/service"
	"github.com/iurnickita/gophermart/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.GetConfig()

	zaplog, err := logger.NewZapLog(cfg.Logger)
	if err != nil {
		return err
	}

	store, err := store.NewStore(cfg.Store)
	if err != nil {
		return err
	}

	auth := auth.NewAuth(store)
	service := service.NewService(cfg.Service, store)

	return handler.Serve(cfg.Handler, auth, service, zaplog)
}
