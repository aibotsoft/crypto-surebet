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
	if order.Data.Status == ftxapi.OrderStatusClosed {
		go p.heal(order.Data)
		p.orderMap.Delete(order.Data.ID)
	}
	err := p.store.SaveOrder(order.Data)
	if err != nil {
		p.log.Error("save_order_error", zap.Error(err))
	}
}
func (p *Placer) GetOpenOrders() error {
	resp, err := p.client.NewGetOpenOrdersService().Do(p.ctx)
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
	//err = p.store.SaveOrders(&data)
	//if err != nil {
	//	return err
	//}
	return err
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
		zap.Duration("api_time", startSave.Sub(start)),
		zap.Duration("save_time", time.Since(startSave)),
		zap.Int("goroutine", runtime.NumGoroutine()),
	)
	return nil
}
func (p *Placer) processFills(fills *ftxapi.WsFillsEvent) {
	fillsCounter.Inc()
	//p.log.Info("fills", zap.Any("data", fills.Data))
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
