package placer

import (
	"context"
	"fmt"
	"github.com/aibotsoft/crypto-surebet/pkg/config"
	"github.com/aibotsoft/crypto-surebet/pkg/store"
	"github.com/aibotsoft/ftx-api"
	"github.com/jinzhu/copier"
	"github.com/nats-io/nats.go"
	"github.com/shopspring/decimal"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"runtime"
	"sync"
	"time"
)

const cryptoSubject = "crypto-surebet"
const usdt = "USDT"

var d100 = decimal.RequireFromString("100")
var d003 = decimal.RequireFromString("0.03")
var d1 = decimal.RequireFromString("1")
var d2 = decimal.RequireFromString("2")
var placeCounter, fillsCounter atomic.Int64

type Placer struct {
	cfg            *config.Config
	log            *zap.Logger
	ctx            context.Context
	store          *store.Store
	nc             *nats.Conn
	ec             *nats.EncodedConn
	client         *ftxapi.Client
	accountInfo    store.Account
	marketMap      map[string]*store.MarketEmb
	marketLock     sync.Mutex
	balanceMap     map[string]*store.BalanceEmb
	balanceLock    sync.Mutex
	symbolMap      map[string]bool
	symbolLock     sync.Mutex
	ws             *ftxapi.WebsocketService
	checkBalanceCh chan bool
	placeConfig    PlaceConfig
	saveSbCh       chan *store.Surebet
	saveOrderCh    chan *ftxapi.Order
	surebetMap     sync.Map
	healMap        sync.Map
	orderMap       sync.Map
}
type PlaceConfig struct {
	MaxStake          decimal.Decimal
	TargetProfit      decimal.Decimal
	TargetAmount      decimal.Decimal
	ReferralRate      decimal.Decimal
	BinFtxVolumeRatio decimal.Decimal
	//ProfitInc    decimal.Decimal
}

func NewPlacer(cfg *config.Config, log *zap.Logger, ctx context.Context, sto *store.Store) (*Placer, error) {
	ftxConfig := ftxapi.Config{
		ApiKey:     cfg.Ftx.Key,
		ApiSecret:  cfg.Ftx.Secret,
		Logger:     log.WithOptions(zap.IncreaseLevel(zap.InfoLevel)).Sugar(),
		SubAccount: ftxapi.StringPointer(cfg.Ftx.SubAccount),
	}
	client := ftxapi.NewClient(ftxConfig)
	ws := ftxapi.NewWebsocketService(cfg.Ftx.Key, cfg.Ftx.Secret, ftxapi.WebsocketEndpoint, log.Sugar()).AutoReconnect()
	ws.SubAccount(cfg.Ftx.SubAccount)
	return &Placer{
		cfg:            cfg,
		log:            log,
		ctx:            ctx,
		store:          sto,
		client:         client,
		ws:             ws,
		marketMap:      make(map[string]*store.MarketEmb),
		balanceMap:     make(map[string]*store.BalanceEmb),
		symbolMap:      make(map[string]bool),
		checkBalanceCh: make(chan bool, 20),
		saveSbCh:       make(chan *store.Surebet, 100),
		saveOrderCh:    make(chan *ftxapi.Order, 100),
		placeConfig: PlaceConfig{
			MaxStake:          decimal.New(cfg.Service.MaxStake, 0),
			TargetProfit:      decimal.NewFromFloat(cfg.Service.TargetProfit),
			BinFtxVolumeRatio: decimal.NewFromFloat(cfg.Service.BinFtxVolumeRatio),
			TargetAmount:      decimal.NewFromFloat(cfg.Service.TargetAmount),
			ReferralRate:      decimal.NewFromFloat(cfg.Service.ReferralRate),
		},
	}, nil
}

func (p *Placer) Close() {
	p.ws.Close()
}
func (p *Placer) Run() error {
	err := p.ws.Connect(p.handler, p.errHandler)
	if err != nil {
		return fmt.Errorf("ws_connect_error: %w", err)
	}
	err = p.ws.Subscribe(ftxapi.Subscription{Channel: ftxapi.WsChannelOrders})
	if err != nil {
		return fmt.Errorf("ws_subscribe_error: %w", err)
	}
	err = p.ws.Subscribe(ftxapi.Subscription{Channel: ftxapi.WsChannelFills})
	if err != nil {
		return fmt.Errorf("ws_subscribe_error: %w", err)
	}
	err = p.AccountInfo()
	if err != nil {
		return err
	}
	err = p.GetBalances()
	if err != nil {
		return err
	}
	err = p.GetOpenOrders()
	if err != nil {
		return fmt.Errorf("get_open_orders_error: %w", err)
	}
	err = p.GetOrdersHistory()
	if err != nil {
		p.log.Warn("get_orders_history_error", zap.Error(err))
		//return err
	}

	err = p.GetMarkets()
	if err != nil {
		return err
	}
	err = p.ConnectAndSubscribe()
	if err != nil {
		return err
	}
	balanceTick := time.Tick(time.Minute * 2)
	marketTick := time.Tick(time.Minute * 5)
	orderTick := time.Tick(time.Minute * 10)
	for {
		select {
		case sb := <-p.saveSbCh:
			err = p.store.SaveSurebet(sb)
			if err != nil {
				p.log.Warn("save_sb_error", zap.Error(err))
			}
		case order := <-p.saveOrderCh:
			err = p.store.SaveOrder(order)
			if err != nil {
				p.log.Warn("save_order_error", zap.Error(err))
			}
		case <-p.checkBalanceCh:
			err := p.GetBalances()
			if err != nil {
				p.log.Info("get_balances_error", zap.Error(err))
			}
		case <-balanceTick:
			_ = p.GetBalances()
		case <-marketTick:
			_ = p.GetMarkets()
		case <-orderTick:
			_ = p.GetOrdersHistory()
		case <-p.ctx.Done():
			p.Close()
			return p.ctx.Err()
		}
	}
}

func (p *Placer) handler(res ftxapi.WsReponse) {
	if res.Orders != nil {
		p.processOrder(res.Orders)
	} else if res.Fills != nil {
		p.processFills(res.Fills)
	}
}

func (p *Placer) errHandler(err error) {
	p.log.Error("ftx_websocket_error", zap.Error(err))
}

func (p *Placer) ConnectAndSubscribe() error {
	url := fmt.Sprintf("nats://%s:%s", p.cfg.Nats.Host, p.cfg.Nats.Port)
	nc, err := nats.Connect(url)
	if err != nil {
		return fmt.Errorf("connect_nats_error: %w", err)
	}
	p.nc = nc
	ec, err := nats.NewEncodedConn(nc, nats.GOB_ENCODER)
	if err != nil {
		return fmt.Errorf("encoded_connection_error: %w", err)
	}
	p.ec = ec
	_, err = ec.Subscribe(cryptoSubject, p.SurebetHandler)
	if err != nil {
		return err
	}
	return nil
}
func (p *Placer) SurebetHandler(sb *store.Surebet) {
	go p.Calc(sb)
}

func (p *Placer) AccountInfo() error {
	start := time.Now()
	resp, err := p.client.NewGetAccountService().Do(p.ctx)
	if err != nil {
		return err
	}
	var data store.Account
	err = copier.Copy(&data, resp)
	if err != nil {
		return err
	}
	err = p.store.SaveAccount(&data)
	if err != nil {
		return err
	}
	p.accountInfo = data
	p.log.Debug("AccountInfo_done",
		zap.Duration("elapsed", time.Since(start)), zap.Int("goroutine", runtime.NumGoroutine()),
	)
	return nil
}
