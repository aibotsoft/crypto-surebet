package config

import (
	"github.com/cristalhq/aconfig"
	"github.com/cristalhq/aconfig/aconfigyaml"
	"time"
)

type Config struct {
	Service struct {
		Name              string  `json:"name"`
		TargetProfit      float64 `json:"target_profit"`
		TargetAmount      int64   `json:"target_amount"`
		BinFtxVolumeRatio int64   `json:"bin_ftx_volume_ratio"`
		ProfitDiffRatio   int64   `json:"profit_diff_ratio"`
		AvgPriceDiffRatio int64   `json:"avg_price_diff_ratio"`
		ProfitIncRatio    int64   `json:"profit_inc_ratio"`
		MaxStake          int64   `json:"max_stake"`
		MinVolume         int64   `json:"min_volume"`
		ReferralRate      float64 `json:"referral_rate"`
		//BinanceMaxDelay     time.Duration `json:"binance_max_delay"`
		//BinanceMaxStaleTime time.Duration `json:"binance_max_stale_time"`
		SendReceiveMaxDelay time.Duration `json:"send_receive_max_delay"`
		MaxLockTime         time.Duration `json:"max_lock_time"`
		ReHealPeriod        time.Duration `json:"re_heal_period"`
		DemoMode            bool          `json:"demo_mode" default:"false"`
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
		WsHost string `json:"ws_host" default:"wss://stream.binance.com:9443/stream?streams="`
		Debug  bool   `json:"debug" default:"false"`
	} `json:"binance"`
	Ftx struct {
		Name       string `json:"name" default:"ftx"`
		WsHost     string `json:"ws_host" default:"wss://ftx.com/ws/"`
		Key        string `json:"-"`
		Secret     string `json:"-"`
		SubAccount string `json:"sub_account"`
	} `json:"ftx"`
	Nats struct {
		Host string `json:"host"`
		Port string `json:"port"`
	} `json:"nats"`
	Ws struct {
		ConnTimeout time.Duration `json:"conn_timeout" default:"5s"`
	} `json:"ws"`
	//Markets  []string
	Postgres struct {
		DSN      string        `json:"dsn"`
		LogLevel string        `json:"log_level"`
		Timeout  time.Duration `json:"timeout" default:"5s"`
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
