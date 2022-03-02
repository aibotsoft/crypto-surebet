package main

import (
	"context"
	"github.com/aibotsoft/crypto-surebet/pkg/config"
	"github.com/aibotsoft/crypto-surebet/pkg/logger"
	"github.com/aibotsoft/crypto-surebet/pkg/signals"
	"github.com/aibotsoft/crypto-surebet/pkg/store"
	"github.com/aibotsoft/crypto-surebet/pkg/version"
	"github.com/aibotsoft/crypto-surebet/services/placer"
	"go.uber.org/zap"
)

func main() {
	cfg := config.NewConfig()
	log, err := logger.NewLogger(cfg.Zap.Level, cfg.Zap.Encoding, cfg.Zap.Caller)
	if err != nil {
		panic(err)
	}
	log.Info("start_service", zap.String("version", version.Version), zap.Any("config", cfg))
	ctx, cancel := context.WithCancel(context.Background())
	sto, err := store.NewStore(cfg, log, ctx)
	if err != nil {
		panic(err)
	}
	err = sto.Migrate()
	if err != nil {
		panic(err)
	}
	//err = sto.Prefetch()
	//if err != nil {
	//	panic(err)
	//}
	p, err := placer.NewPlacer(cfg, log, ctx, sto)
	if err != nil {
		panic(err)
	}
	errCh := make(chan error)
	go func() {
		errCh <- p.Run()
	}()
	defer func() {
		log.Info("closing_services...")
		cancel()
		p.Close()
		err2 := sto.Close()
		if err2 != nil {
			log.Warn("close_db_error", zap.Error(err))
		}
		_ = log.Sync()
	}()
	stopCh := signals.SetupSignalHandler()
	select {
	case err := <-errCh:
		log.Error("stop_service_by_error", zap.Error(err))
	case sig := <-stopCh:
		log.Info("stop_service_by_os_signal", zap.String("signal", sig.String()))
	}
}
