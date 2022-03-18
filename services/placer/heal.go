package placer

import (
	"fmt"
	"github.com/aibotsoft/crypto-surebet/pkg/store"
	"github.com/aibotsoft/ftx-api"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"strings"
	"time"
)

const (
	BET  = "b"
	HEAL = "h"
)

func (p *Placer) placeHeal(h *store.Heal) {
	p.log.Info("begin_heal",
		zap.Any("place", h.PlaceParams),
		zap.Any("filled_size", h.FilledSize),
		zap.Any("avg_fill_price", h.AvgFillPrice),
		zap.Any("fee_part", h.FeePart),
		zap.Any("profit_part", h.ProfitPart),
	)
	for i := 0; i < 5; i++ {
		resp, err := p.PlaceOrder(p.ctx, h.PlaceParams)
		if err != nil {
			p.log.Error("heal_place_error", zap.Error(err))
			msg := fmt.Sprintf("try_count:%d err:%s", i, err.Error())
			h.ErrorMsg = ftxapi.StringPointer(msg)
		}
		if resp != nil {
			h.OrderID = resp.ID
			break
		}
	}

	h.Done = time.Now().UnixNano()

	err := p.store.SaveHeal(h)
	if err != nil {
		p.log.Error("save_heal_error", zap.Error(err))
	}
	p.log.Info("done_heal", zap.Duration("elapsed", time.Duration(h.Done-h.Start)))
}

func (p *Placer) heal(order ftxapi.WsOrders) {
	if strings.Index(*order.ClientID, "h") > 0 {
		if order.FilledSize != 0 {
			return
		}
		p.log.Error("heal_order_zero", zap.Any("order", order))
		got, ok := p.healMap.Load(*order.ClientID)
		if !ok {
			p.log.Warn("not_found_heal_in_map", zap.Any("order", order))
			return
		}
		h := got.(store.Heal)
		//0.01% from original price
		priceInc := h.PlaceParams.Price.Div(d100).Div(d100)
		if h.PlaceParams.Side == store.SideSell {
			h.PlaceParams.Price = h.PlaceParams.Price.Add(priceInc)
		} else {
			h.PlaceParams.Price = h.PlaceParams.Price.Sub(priceInc)
		}
		if h.ErrorMsg != nil {
			newMsg := fmt.Sprintf("heal_price_inc:%v", priceInc)
			msg := fmt.Sprintf("%s :: %s", newMsg, *h.ErrorMsg)
			h.ErrorMsg = ftxapi.StringPointer(msg)
		} else {
			msg := fmt.Sprintf("heal_price_inc:%v", priceInc)
			h.ErrorMsg = ftxapi.StringPointer(msg)
		}
		p.placeHeal(&h)
		return

	} else if order.FilledSize == 0 {
		p.surebetMap.Delete(*order.ClientID)
		return
	}
	got, ok := p.surebetMap.Load(*order.ClientID)
	if !ok {
		p.log.Warn("not_found_surebet_in_map", zap.Any("order", order))
		return
	}
	sb := got.(*store.Surebet)
	h := store.Heal{
		ID:           sb.ID,
		Start:        time.Now().UnixNano(),
		FilledSize:   decimal.NewFromFloat(order.FilledSize),
		AvgFillPrice: decimal.NewFromFloat(order.AvgFillPrice),
		PlaceParams: store.PlaceParamsEmb{
			Market:   sb.PlaceParams.Market,
			Type:     store.OrderTypeLimit,
			Ioc:      false,
			PostOnly: true,
			ClientID: fmt.Sprintf("%d:%s", sb.ID, HEAL),
		},
	}

	h.FeePart = h.AvgFillPrice.Mul(h.FilledSize).Mul(sb.RealFee).Div(d100)
	h.ProfitPart = h.AvgFillPrice.Mul(h.FilledSize).Mul(sb.TargetProfit).Div(d100)
	h.PlaceParams.Size = h.FilledSize

	if sb.PlaceParams.Side == store.SideSell {
		h.PlaceParams.Side = store.SideBuy
		h.PlaceParams.Price = h.AvgFillPrice.Mul(h.FilledSize).Sub(h.FeePart).Sub(h.ProfitPart).Div(h.PlaceParams.Size)
	} else {
		h.PlaceParams.Side = store.SideSell
		h.PlaceParams.Price = h.AvgFillPrice.Mul(h.FilledSize).Add(h.FeePart).Add(h.ProfitPart).Div(h.PlaceParams.Size)
	}
	p.healMap.Store(h.PlaceParams.ClientID, h)
	p.placeHeal(&h)
}
