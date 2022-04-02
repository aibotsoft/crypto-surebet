package placer

import (
	"fmt"
	"github.com/aibotsoft/crypto-surebet/pkg/store"
	"github.com/aibotsoft/ftx-api"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"time"
)

func (p *Placer) placeHeal(h *store.Heal) {
	p.healMap.Store(h.ID, h)

	for i := 0; i < 10; i++ {
		resp, err := p.PlaceOrder(p.ctx, h.PlaceParams)
		if err != nil {
			p.log.Error("heal_place_error", zap.Error(err))
			msg := fmt.Sprintf("try:%d err:%s", i, err.Error())
			if h.ErrorMsg != nil {
				msg = fmt.Sprintf("%s :: %s", msg, *h.ErrorMsg)
			}
			h.ErrorMsg = ftxapi.StringPointer(msg)
		}
		if resp != nil {
			h.Orders = append(h.Orders, resp)
			//h.OrderID = resp.ID
			break
		}
	}
	h.Done = time.Now().UnixNano()
	p.log.Info("heal",
		zap.Any("id", h.ID),
		zap.Any("m", h.PlaceParams.Market),
		zap.Any("s", h.PlaceParams.Side),
		zap.Any("heal_count", len(h.Orders)),
		zap.Any("pr", h.PlaceParams.Price),
		zap.Any("sz", h.PlaceParams.Size),
		zap.Any("v", h.PlaceParams.Size.Mul(h.PlaceParams.Price).Floor()),
		zap.Any("p_part", h.ProfitPart),
		zap.Any("msg", h.ErrorMsg),
		zap.Any("c_id", h.PlaceParams.ClientID),
		zap.Int64("el", (h.Done-h.ID)/1000000),
	)
	p.saveHealCh <- h
	p.log.Debug("done_heal", zap.Duration("elapsed", time.Duration(h.Done-h.Start)))
}

func (p *Placer) reHeal(order store.Order, clientID ClientID) {
	if order.FilledSize != 0 {
		return
	}
	p.log.Error("heal_order_zero", zap.Any("order", order))
	got, ok := p.healMap.Load(clientID.ID)
	if !ok {
		p.log.Warn("not_found_heal_in_map", zap.Any("order", order))
		return
	}
	h := got.(*store.Heal)
	clientID.Try = clientID.Try + 1
	h.PlaceParams.ClientID = marshalClientID(clientID)

	//TargetProfit*2 from original price
	priceInc := h.PlaceParams.Price.Div(d100).Mul(p.placeConfig.TargetProfit.Mul(d2))
	if h.PlaceParams.Side == store.SideSell {
		h.PlaceParams.Price = h.PlaceParams.Price.Add(priceInc)
	} else {
		h.PlaceParams.Price = h.PlaceParams.Price.Sub(priceInc)
	}
	msg := fmt.Sprintf("heal_price_inc:%v", priceInc)
	if h.ErrorMsg != nil {
		msg = fmt.Sprintf("%s :: %s", msg, *h.ErrorMsg)
	}
	h.ErrorMsg = ftxapi.StringPointer(msg)
	p.placeHeal(h)
}

func (p *Placer) heal(order store.Order, clientID ClientID) {
	got, ok := p.surebetMap.LoadAndDelete(clientID.ID)
	if !ok {
		p.log.Warn("not_found_surebet_in_map", zap.Any("order", order))
		return
	}
	lock := p.Lock(symbolFromMarket(order.Market))
	defer func() {
		//p.surebetMap.Delete(clientID.ID)
		id := <-lock
		p.log.Debug("unlock", zap.Int64("id", id), zap.String("m", order.Market), zap.Int64("elapsed", (time.Now().UnixNano()-id)/1000000))
	}()
	if order.FilledSize == 0 {
		p.deleteSbCh <- order.ID
		return
	}
	sb := got.(*store.Surebet)
	h := &store.Heal{
		ID:           sb.ID,
		Start:        time.Now().UnixNano(),
		FilledSize:   decimal.NewFromFloat(order.FilledSize),
		AvgFillPrice: decimal.NewFromFloat(order.AvgFillPrice),
		PlaceParams: store.PlaceParamsEmb{
			Market:   sb.PlaceParams.Market,
			Type:     store.OrderTypeLimit,
			Ioc:      false,
			PostOnly: true,
			//ClientID: fmt.Sprintf("%d:%s:0", sb.ID, HEAL),
			ClientID: marshalClientID(ClientID{
				ID:   sb.ID,
				Side: HEAL,
				Try:  0,
			}),
		},
	}

	h.FeePart = h.AvgFillPrice.Mul(h.FilledSize).Mul(sb.RealFee).Div(d100)
	h.ProfitPart = h.AvgFillPrice.Mul(h.FilledSize).Mul(sb.TargetProfit).Div(d100)
	h.PlaceParams.Size = h.FilledSize

	if sb.PlaceParams.Side == store.SideSell {
		h.PlaceParams.Side = store.SideBuy
		price := h.AvgFillPrice.Mul(h.FilledSize).Sub(h.FeePart).Sub(h.ProfitPart).Div(h.PlaceParams.Size)
		h.PlaceParams.Price = price.Div(sb.Market.PriceIncrement).Floor().Mul(sb.Market.PriceIncrement)
	} else {
		h.PlaceParams.Side = store.SideSell
		price := h.AvgFillPrice.Mul(h.FilledSize).Add(h.FeePart).Add(h.ProfitPart).Div(h.PlaceParams.Size)
		h.PlaceParams.Price = price.Div(sb.Market.PriceIncrement).Floor().Mul(sb.Market.PriceIncrement)
	}
	if h.PlaceParams.Size.LessThan(sb.Market.MinProvideSize) {
		p.log.Warn("size_too_small_to_heal", zap.Any("h", h))
		msg := fmt.Sprintf("size:%v min_provide:%v", h.PlaceParams.Size, sb.Market.MinProvideSize)
		h.ErrorMsg = ftxapi.StringPointer(msg)
		h.Done = time.Now().UnixNano()
		h.ProfitPart = decimal.Zero
		p.saveHealCh <- h
		return
	}
	p.placeHeal(h)
}
