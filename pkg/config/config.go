package config

import (
	"github.com/cristalhq/aconfig"
	"github.com/cristalhq/aconfig/aconfigyaml"
	"time"
)

type Config struct {
	Service struct {
		Name         string  `json:"name"`
		TargetProfit float64 `json:"target_profit"`
		//TargetAmount float64
		//MinProfit    float64
		//ProfitInc    float64
		BinFtxVolumeRatio   float64       `json:"bin_ftx_volume_ratio"`
		MaxStake            int64         `json:"max_stake"`
		ReferralRate        float64       `json:"referral_rate"`
		BinanceMaxDelay     time.Duration `json:"binance_max_delay"`
		BinanceMaxStaleTime time.Duration `json:"binance_max_stale_time"`
		SendReceiveMaxDelay time.Duration `json:"send_receive_max_delay"`
	} `json:"service"`
	Zap struct {
		//debug, info, warn, error, fatal, panic
		Level string `json:"level" default:"info"`
		//console, json
		Encoding string `json:"encoding" default:"console"`
		//disable, short, full
		Caller string `json:"caller" default:"short"`
	} `json:"zap"`
	Binance struct {
		Name string `json:"name" default:"binance"`
		//WsHost string `default:"wss://stream.binance.com:9443/ws"`
		WsHost string `json:"wsHost" default:"wss://stream.binance.com:9443/stream?streams="`
		Debug  bool   `json:"debug" default:"false"`
	} `json:"binance"`
	Ftx struct {
		Name       string `json:"name" default:"ftx"`
		WsHost     string `json:"wsHost" default:"wss://ftx.com/ws/"`
		Key        string `json:"key"`
		Secret     string `json:"secret"`
		SubAccount string `json:"sub_account"`
	} `json:"ftx"`
	Nats struct {
		Host string `json:"host"`
		Port string `json:"port"`
	} `json:"nats"`
	Ws struct {
		ConnTimeout time.Duration `json:"connTimeout" default:"5s"`
	} `json:"ws"`
	//Markets  []string
	Postgres struct {
		DSN      string `json:"dsn"`
		LogLevel string `json:"log_level"`
	} `json:"postgres"`
}

func NewConfig() *Config {
	var cfg Config
	loader := aconfig.LoaderFor(&cfg, aconfig.Config{
		SkipFlags:          true,
		AllFieldRequired:   true,
		AllowUnknownFlags:  true,
		AllowUnknownEnvs:   true,
		AllowUnknownFields: true,
		AllowDuplicates:    true,
		SkipEnv:            false,
		FileFlag:           "config",
		FailOnFileNotFound: false,
		MergeFiles:         true,
		Files:              []string{"config.yaml", "crypto-surebet.yaml"},
		FileDecoders: map[string]aconfig.FileDecoder{
			".yaml": aconfigyaml.New(),
		},
	})
	err := loader.Load()
	if err != nil {
		panic(err)
	}

	return &cfg
}
