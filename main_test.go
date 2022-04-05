package main

import (
	"context"
	"fmt"
	"github.com/aibotsoft/crypto-surebet/pkg/config"
	"github.com/aibotsoft/crypto-surebet/pkg/logger"
	"github.com/aibotsoft/crypto-surebet/pkg/store"
	"github.com/aibotsoft/crypto-surebet/services/placer"
	"testing"
)

var p *placer.Placer

func TestMain(m *testing.M) {
	cfg := config.NewConfig()
	log, err := logger.NewLogger(cfg.Zap.Level, cfg.Zap.Encoding, cfg.Zap.Caller)
	if err != nil {
		panic(err)
	}
	//log.Info("start_service", zap.Any("config", cfg))
	ctx, cancel := context.WithCancel(context.Background())
	sto, err := store.NewStore(cfg, log, ctx)
	if err != nil {
		panic(err)
	}
	err = sto.Migrate()
	if err != nil {
		panic(err)
	}
	p, err = placer.NewPlacer(cfg, log, ctx, sto)
	if err != nil {
		panic(err)
	}
	m.Run()
	//log.Info("closing...")
	cancel()
	//p.Close()
	sto.Close()
}

func Test_AccountInfo(t *testing.T) {
	got := p.FindHeal(0, true)
	fmt.Printf("%+v", got)
}
