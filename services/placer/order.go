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
		return nil, err
	}
	//p.log.Info("place_done",
	//	zap.Duration("elapsed", time.Since(start)),
	//	zap.Any("params", data),
	//	//zap.Any("resp", resp),
	//)
	//fmt.Println(resp)
	return resp, nil

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

	if o.Status == store.OrderStatusClosed {
		go p.heal(order.Data)
		p.orderMap.Delete(o.ID)
		o.ClosedAt = ftxapi.Int64Pointer(time.Now().UnixNano())
	} else {
		p.orderMap.Store(o.ID, o)
	}
	err = p.store.SaveOrder(&o)
	if err != nil {
		p.log.Error("save_order_error", zap.Error(err))
	}
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
		p.orderMap.Store(order.ID, order)
		//p.log.Info("", zap.Any("order", order))
	}
	return err
}
func (p *Placer) GetOpenBuySell(coin string) (decimal.Decimal, decimal.Decimal) {
	var buy, sell decimal.Decimal
	p.orderMap.Range(func(key, value interface{}) bool {
		order := value.(store.Order)
		if strings.Index(order.Market, coin) == -1 {
			return true
		}
		if order.Status == store.OrderStatusClosed {
			p.orderMap.Delete(key)
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
	err = p.store.SaveFills(&data)
	if err != nil {
		p.log.Warn("save_fills_error", zap.Error(err))
		return
	}
}
