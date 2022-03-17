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
	"go.uber.org/zap"
	"runtime"
	"sync"
	"time"
)

const cryptoSubject = "crypto-surebet"

var d100 = decimal.RequireFromString("100")
var d1 = decimal.RequireFromString("1")

type Placer struct {
	cfg         *config.Config
	log         *zap.Logger
	ctx         context.Context
	store       *store.Store
	nc          *nats.Conn
	ec          *nats.EncodedConn
	client      *ftxapi.Client
	accountInfo store.Account
	marketMap   map[string]*store.MarketEmb
	marketLock  sync.Mutex
	balanceMap  map[string]*store.BalanceEmb
	balanceLock sync.Mutex

	symbolMap      map[string]bool
	symbolLock     sync.Mutex
	ws             *ftxapi.WebsocketService
	checkBalanceCh chan bool
	placeConfig    PlaceConfig
	targetAmount   decimal.Decimal
	saveSbCh       chan *store.Surebet
	saveOrderCh    chan *ftxapi.Order
	surebetMap     sync.Map
}
type PlaceConfig struct {
	MaxStake     decimal.Decimal
	TargetProfit decimal.Decimal
	//TargetAmount decimal.Decimal
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
			//TargetAmount: decimal.NewFromFloat(cfg.Service.TargetAmount),
			ReferralRate: decimal.NewFromFloat(cfg.Service.ReferralRate),
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
			p.GetBalances()
		case <-marketTick:
			p.GetMarkets()
		case <-orderTick:
			p.GetOrdersHistory()
		case <-p.ctx.Done():
			p.Close()
			return p.ctx.Err()
		}
	}
}

func (p *Placer) errHandler(err error) {
	p.log.Error("ftx_websocket_error", zap.Error(err))
}

func (p *Placer) PlaceOrder(ctx context.Context, param store.PlaceParamsEmb) (*ftxapi.Order, error) {
	//start := time.Now()
	data := ftxapi.PlaceOrderParams{
		Market:   param.Market,
		Side:     ftxapi.Side(param.Side),
		Price:    DecimalToFloat64(param.Price),
		Type:     ftxapi.OrderType(param.Type),
		Size:     DecimalToFloat64(param.Size),
		Ioc:      &param.Ioc,
		PostOnly: &param.PostOnly,
		ClientID: ftxapi.StringPointer(param.ClientID),
	}
	//p.log.Info("params", zap.Any("p", data))
	resp, err := p.client.NewPlaceOrderService().Params(data).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("place_error: %w", err)
	}
	//p.log.Info("place_done",
	//	zap.Duration("elapsed", time.Since(start)),
	//	zap.Any("params", data),
	//	//zap.Any("resp", resp),
	//)
	//fmt.Println(resp)
	return resp, nil

}
func (p *Placer) processFills(fills *ftxapi.WsFillsEvent) {
	fillsCounter.Inc()
	p.log.Info("fills", zap.Any("data", fills.Data))
	var data store.Fills
	err := copier.Copy(&data, fills.Data)
	if err != nil {
		p.log.Warn("copy_fills_error", zap.Error(err))
		return
	}
	err = p.store.SaveFills(&data)
	if err != nil {
		p.log.Warn("save_fills_error", zap.Error(err))
		return
	}
}
func (p *Placer) processOrder(order *ftxapi.WsOrdersEvent) {
	if order.Data.ClientID == nil {
		p.log.Info("order_client_id_null", zap.Any("data", order.Data))
		return
	}
	if order.Data.Status == ftxapi.OrderStatusClosed {
		go p.heal(order.Data)
	}
	err := p.store.SaveOrder(order.Data)
	if err != nil {
		p.log.Error("save_order_error", zap.Error(err))
	}
}
func (p *Placer) handler(res ftxapi.WsReponse) {
	if res.Orders != nil {
		p.processOrder(res.Orders)
	} else if res.Fills != nil {
		p.processFills(res.Fills)
	}
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
func (p *Placer) GetMarkets() error {
	//start := time.Now()
	resp, err := p.client.NewGetMarketsService().Do(p.ctx)
	if err != nil {
		return err
	}
	//fmt.Println(resp)
	var data []store.Market
	err = copier.Copy(&data, resp)
	if err != nil {
		return err
	}
	p.saveMarkets(data)
	err = p.store.SaveMarkets(&data)
	if err != nil {
		return err
	}
	//p.log.Debug("GetMarkets_done",
	//	zap.Int("count", len(data)),
	//	zap.Duration("elapsed", time.Since(start)),
	//	zap.Int("goroutine", runtime.NumGoroutine()))
	return nil
}

func (p *Placer) GetOrdersHistory() error {
	start := time.Now()
	resp, _, err := p.client.NewGetOrderHistoryService().Do(p.ctx)
	if err != nil {
		return fmt.Errorf("GetOrdersHistory_error: %w", err)
	}
	if len(resp) == 0 {
		return fmt.Errorf("order_list_empty")
	}
	startSave := time.Now()
	err = p.store.SaveOrders(resp)
	if err != nil {
		p.log.Error("save_order_error", zap.Error(err))
	}
	//for i := 0; i < len(resp); i++ {
	//	err := p.store.SaveOrder(resp[i])
	//	if err != nil {
	//		p.log.Error("save_order_error", zap.Error(err), zap.Any("order", resp[i]))
	//	}
	//}
	//var data []store.Order
	//err = copier.Copy(&data, resp)
	//if err != nil {
	//	return err
	//}
	//for i := 0; i < len(data); i++ {
	//	if data[i].ClientID != nil {
	//		var clientID store.OrderInfo
	//		err := json.Unmarshal([]byte(*data[i].ClientID), &clientID)
	//		if err != nil {
	//			sbId, err := strconv.ParseInt(*data[i].ClientID, 10, 64)
	//			if err != nil {
	//				continue
	//			}
	//			data[i].SbID = sbId
	//		}
	//		data[i].SbID = clientID.SbID
	//	}
	//
	//	//p.log.Info("", zap.Any("", data[i]))
	//}
	//err = p.store.SaveOrders(&data)
	//if err != nil {
	//	return err
	//}
	p.log.Debug("get_orders_done",
		zap.Int("count", len(resp)),
		//zap.Bool("has_more", b),
		zap.Duration("api_time", startSave.Sub(start)),
		zap.Duration("save_time", time.Since(startSave)),
		zap.Int("goroutine", runtime.NumGoroutine()),
	)
	return nil
}

//func (p *Placer) GetOpenOrders() error {
//	resp, err := p.client.NewGetOpenOrdersService().Do(p.ctx)
//	if err != nil {
//		return err
//	}
//	var data []store.Order
//	err = copier.Copy(&data, resp)
//	if err != nil {
//		return err
//	}
//	err = p.store.SaveOrders(&data)
//	if err != nil {
//		return err
//	}
//	return err
//}
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
func (p *Placer) GetBalances() error {
	//start := time.Now()
	//var Reused, WasIdle bool
	//var IdleTime time.Duration
	//trace := &httptrace.ClientTrace{
	//	GotConn: func(connInfo httptrace.GotConnInfo) {
	//		//fmt.Printf("Got Conn: %+v\n", connInfo)
	//		Reused = connInfo.Reused
	//		WasIdle = connInfo.WasIdle
	//		IdleTime = connInfo.IdleTime
	//	},
	//	//DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
	//	//	fmt.Printf("DNS Info: %+v\n", dnsInfo)
	//	//},
	//}
	//ctx := httptrace.WithClientTrace(p.ctx, trace)
	resp, err := p.client.NewGetBalancesService().Do(p.ctx)
	if err != nil {
		return err
	}
	if len(resp) == 0 {
		return fmt.Errorf("balance_list_empty")
	}
	//p.log.Info("balance_resp", zap.Any("resp", resp))
	var data []store.Balance
	err = copier.Copy(&data, resp)
	if err != nil {
		return err
	}
	p.saveBalances(data)
	//total := p.BalanceTotal()
	err = p.store.SaveBalances(&data)
	if err != nil {
		return err
	}
	//p.log.Debug("GetBalances_done",
	//	zap.Int("count", len(data)),
	//	//zap.Int64("total", total.IntPart()),
	//	zap.Any("p.targetAmount", p.targetAmount),
	//	//zap.Duration("elapsed", time.Since(start)),
	//	zap.Bool("reused", Reused),
	//	zap.Bool("was_idle", WasIdle),
	//	zap.Duration("idle_time", IdleTime),
	//	zap.Int("goroutine", runtime.NumGoroutine()),
	//)
	return nil
}

//func (p *Placer) BalanceTotal() decimal.Decimal {
//	var total decimal.Decimal
//	for _, bal := range p.balanceMap {
//		total = total.Add(bal.UsdValue)
//	}
//	return total
//}
func (p *Placer) BalanceAdd(coin string, usdValueDiff decimal.Decimal, coinValueDiff decimal.Decimal) {
	p.balanceLock.Lock()
	defer p.balanceLock.Unlock()
	got, ok := p.balanceMap[coin]
	if !ok {
		return
	}
	got.UsdValue = got.UsdValue.Add(usdValueDiff)
	got.Free = got.Free.Add(coinValueDiff)
}
func (p *Placer) BalanceSub(coin string, usdValueDiff decimal.Decimal, coinValueDiff decimal.Decimal) {
	p.balanceLock.Lock()
	defer p.balanceLock.Unlock()
	got, ok := p.balanceMap[coin]
	if !ok {
		return
	}
	got.UsdValue = got.UsdValue.Sub(usdValueDiff)
	got.Free = got.Free.Sub(coinValueDiff)
}
func (p *Placer) FindBalance(coin string) *store.BalanceEmb {
	p.balanceLock.Lock()
	defer p.balanceLock.Unlock()
	got, ok := p.balanceMap[coin]
	if !ok {
		return &store.BalanceEmb{}
	}
	return got
}
func (p *Placer) saveBalances(data []store.Balance) {
	p.balanceLock.Lock()
	defer p.balanceLock.Unlock()
	var total decimal.Decimal
	var count decimal.Decimal
	for _, b := range data {
		p.balanceMap[b.Coin] = &store.BalanceEmb{
			Free:     b.Free,
			Total:    b.Total,
			UsdValue: b.UsdValue,
		}
		total = total.Add(b.UsdValue)
		if b.UsdValue.IsZero() || b.Coin == "USD" || b.Coin == "USDT" {
			continue
		}
		count = count.Add(d1)
	}
	p.targetAmount = total.Div(count).DivRound(p.placeConfig.MaxStake, 2)
	//p.log.Info("balance",
	//	//zap.Any("targetAmount", p.targetAmount),
	//	zap.Any("targetAmount", p.targetAmount),
	//	zap.Any("total", total),
	//	zap.Any("len(data)", len(data)),
	//	zap.Any("count", count),
	//)
}
func (p *Placer) saveMarkets(data []store.Market) {
	p.marketLock.Lock()
	defer p.marketLock.Unlock()
	for _, m := range data {
		var me store.MarketEmb
		_ = copier.Copy(&me, m)
		p.marketMap[m.Name] = &me
	}
}
func (p *Placer) FindMarket(symbol string) *store.MarketEmb {
	p.marketLock.Lock()
	defer p.marketLock.Unlock()
	return p.marketMap[symbol]
}

func (p *Placer) Unlock(symbol string) {
	p.symbolLock.Lock()
	defer p.symbolLock.Unlock()
	p.symbolMap[symbol] = false
}
func (p *Placer) Lock(symbol string) bool {
	p.symbolLock.Lock()
	defer p.symbolLock.Unlock()
	got, ok := p.symbolMap[symbol]
	if !ok || !got {
		//fmt.Println("new_symbol", symbol)
		p.symbolMap[symbol] = true
		return true
	}
	return false
}

//func (p *Placer) CancelOrderByClientID(clientID string) {
//	var Reused, WasIdle bool
//	var IdleTime time.Duration
//	trace := &httptrace.ClientTrace{
//		GotConn: func(connInfo httptrace.GotConnInfo) {
//			//fmt.Printf("Got Conn: %+v\n", connInfo)
//			Reused = connInfo.Reused
//			WasIdle = connInfo.WasIdle
//			IdleTime = connInfo.IdleTime
//		},
//	}
//	err := p.client.NewCancelOrderByClientIDService().ClientID(clientID).Do(httptrace.WithClientTrace(p.ctx, trace))
//	if err != nil {
//		p.log.Error("cancel_order_error", zap.Error(err), zap.Any("clientID", clientID))
//	} else {
//		p.log.Info("cancel_order_send",
//			zap.String("clientID", clientID),
//			zap.Bool("reused", Reused),
//			zap.Bool("was_idle", WasIdle),
//			zap.Duration("idle_time", IdleTime),
//			zap.Int("goroutine", runtime.NumGoroutine()),
//		)
//	}
//}
