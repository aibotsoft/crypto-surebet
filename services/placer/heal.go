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

func (p *Placer) heal(order ftxapi.WsOrders) {
	if strings.Index(*order.ClientID, "h") > 0 {
		if order.FilledSize != 0 {
			return
		}
		p.log.Error("heal_order_zero", zap.Any("order", order))
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
		ID:    sb.ID,
		Start: time.Now().UnixNano(),
	}

	h.FilledSize = decimal.NewFromFloat(order.FilledSize)
	h.AvgFillPrice = decimal.NewFromFloat(order.AvgFillPrice)

	h.FeePart = h.AvgFillPrice.Mul(h.FilledSize).Mul(sb.RealFee).Div(d100)

	h.ProfitPart = h.AvgFillPrice.Mul(h.FilledSize).Mul(sb.TargetProfit).Div(d100)

	h.PlaceParams.Size = h.FilledSize

	if order.Side == ftxapi.SideSell {
		h.PlaceParams.Side = store.SideBuy
		h.PlaceParams.Price = h.AvgFillPrice.Mul(h.FilledSize).Sub(h.FeePart).Sub(h.ProfitPart).Div(h.PlaceParams.Size)
	} else {
		h.PlaceParams.Side = store.SideSell
		h.PlaceParams.Price = h.AvgFillPrice.Mul(h.FilledSize).Add(h.FeePart).Add(h.ProfitPart).Div(h.PlaceParams.Size)
	}

	h.PlaceParams.Market = order.Market
	h.PlaceParams.Type = store.OrderTypeLimit
	h.PlaceParams.Ioc = false
	h.PlaceParams.PostOnly = true
	h.PlaceParams.ClientID = fmt.Sprintf("%d:%s", sb.ID, HEAL)
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
			p.log.Error("heal_place_error",
				zap.Error(err),
				zap.Duration("elapsed", time.Duration(time.Now().UnixNano()-sb.StartTime)),
			)
			msg := fmt.Sprintf("try_count:%d err:%s", i, err.Error())
			h.ErrorMsg = ftxapi.StringPointer(msg)
		}
		if resp != nil {
			h.OrderID = resp.ID
			break
		}
	}

	h.Done = time.Now().UnixNano()

	err := p.store.SaveHeal(&h)
	if err != nil {
		p.log.Error("save_heal_error", zap.Error(err))
	}
	p.log.Info("done_heal",
		zap.Duration("elapsed", time.Duration(h.Done-h.Start)),
	)
}
