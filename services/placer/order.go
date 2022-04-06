package placer

import (
	"context"
	"fmt"
	"github.com/aibotsoft/crypto-surebet/pkg/store"
	ftxapi "github.com/aibotsoft/ftx-api"
	"github.com/jinzhu/copier"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"runtime"
	"strings"
	"time"
)

func DecimalToFloat64(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}
func (p *Placer) PlaceOrder(ctx context.Context, param store.PlaceParamsEmb) (*store.Order, error) {
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
		return nil, err
	}
	var o store.Order
	err = copier.Copy(&o, resp)
	if err != nil {
		return nil, err
	}
	//p.log.Info("place_done",
	//	zap.Duration("elapsed", time.Since(start)),
	//	zap.Any("params", data),
	//	//zap.Any("resp", resp),
	//)
	//fmt.Println(resp)
	return &o, nil

}

func (p *Placer) processOrder(order *ftxapi.WsOrdersEvent) {
	if order.Data.ClientID == nil {
		p.log.Info("order_client_id_null", zap.Any("data", order.Data))
		return
	}
	var o store.Order
	err := copier.Copy(&o, order.Data)
	if err != nil {
		p.log.Error("copy_order_error", zap.Error(err))
		return
	}
	clientID, err := unmarshalClientID(*o.ClientID)
	if err != nil {
		return
	}

	if o.PostOnly == true {
		sumSize := o.RemainingSize + o.FilledSize
		if o.Size > sumSize && sumSize > 0 {
			p.log.Error("order_size_reduce", zap.Any("order", o))
		}
	}
	if o.Status == store.OrderStatusClosed {
		p.openOrderMap.Delete(o.ID)
		o.ClosedAt = ftxapi.Int64Pointer(time.Now().UnixNano())
		if clientID.Side == BET {
			go p.heal(o, clientID)
		} else {
			go p.reHeal(o, clientID)
		}

	} else {
		p.openOrderMap.Store(o.ID, o)
	}
	p.store.SaveOrder(&o)
}
func (p *Placer) GetOpenOrders() error {
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()
	resp, err := p.client.NewGetOpenOrdersService().Do(ctx)
	if err != nil {
		return err
	}
	var data []store.Order
	err = copier.Copy(&data, resp)
	if err != nil {
		return err
	}
	for _, order := range data {
		p.openOrderMap.Store(order.ID, order)
		p.openOrderCh <- order
	}
	return err
}
func (p *Placer) GetOpenBuySell(coin string) (decimal.Decimal, decimal.Decimal) {
	var buy, sell decimal.Decimal
	p.openOrderMap.Range(func(key, value interface{}) bool {
		order := value.(store.Order)
		if strings.Index(order.Market, coin) == -1 {
			return true
		}
		if order.Status == store.OrderStatusClosed {
			p.openOrderMap.Delete(key)
			return true
		}
		if order.Side == store.SideBuy {
			buy = buy.Add(decimal.NewFromFloat(order.Size))
		} else {
			sell = sell.Add(decimal.NewFromFloat(order.Size))
		}
		//fmt.Println(key, order, coin, buy, sell)
		return true
	})
	return buy, sell
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
	p.log.Debug("get_orders_done",
		zap.Int("count", len(resp)),
		//zap.Bool("has_more", b),
		zap.Int64("api_time_ms", startSave.Sub(start).Milliseconds()),
		zap.Int64("save_time_ms", time.Since(startSave).Milliseconds()),
		zap.Int("goroutine", runtime.NumGoroutine()),
	)
	return nil
}
func (p *Placer) processFills(fills *ftxapi.WsFillsEvent) {
	p.log.Debug("fills", zap.Any("data", fills.Data))
	var data store.Fills
	err := copier.Copy(&data, fills.Data)
	if err != nil {
		p.log.Warn("copy_fills_error", zap.Error(err))
		return
	}
	p.saveFillsCh <- &data
}
func (p *Placer) processOpenOrder(order *store.Order) {
	if order.ClientID == nil {
		return
	}
	if time.Since(order.CreatedAt) < p.cfg.Service.ReHealPeriod {
		return
	}
	clientID, err := unmarshalClientID(*order.ClientID)
	if err != nil {
		return
	}
	heal := p.FindHeal(clientID.ID, false)
	if heal == nil {
		return
	}
	got, ok := p.lastFtxPriceMap.Load(order.Market)
	if !ok {
		return
	}
	lastPrice := got.(decimal.Decimal)
	//percentage difference = 100 * |a - b| / ((a + b) / 2)
	price := decimal.NewFromFloat(order.Price)
	percentDiff := price.Sub(lastPrice).Abs().Div(price.Add(lastPrice).Div(d2)).Mul(d100)
	p.log.Info("close_stale",
		zap.Int64("i", heal.ID),
		zap.String("m", order.Market),
		zap.String("s", string(order.Side)),
		zap.Float64("pr", order.Price),
		zap.Any("last_price", lastPrice),
		zap.Any("percent_diff", percentDiff),
		zap.Float64("sz", order.Size),
		zap.Duration("since", time.Since(order.CreatedAt)),
		//zap.Int("order_count", len(heal.Orders)),
		//zap.Any("clientID", clientID),
		zap.Int64("order_id", order.ID),
		//zap.Any("heal", heal),
	)
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()
	err = p.client.NewCancelOrderService().OrderID(order.ID).Do(ctx)
	if err != nil {
		p.log.Error("cancel_order_error", zap.Error(err))
	}
}
