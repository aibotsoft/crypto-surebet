package placer

import (
	"fmt"
	"github.com/aibotsoft/crypto-surebet/pkg/store"
	"github.com/aibotsoft/ftx-api"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"time"
)

const million = 1000000

func (p *Placer) placeHeal(h *store.Heal) {
	p.healMap.Store(h.ID, h)

	for i := 0; i < 10; i++ {
		resp, err := p.PlaceOrder(p.ctx, h.PlaceParams)
		if err != nil {
			p.log.Error("heal_error", zap.Error(err))
			msg := fmt.Sprintf("try:%d err:%s", i, err.Error())
			if h.ErrorMsg != nil {
				msg = fmt.Sprintf("%s :: %s", msg, *h.ErrorMsg)
			}
			h.ErrorMsg = ftxapi.StringPointer(msg)
		}
		if resp != nil {
			h.Orders = append(h.Orders, resp)
			break
		}
	}
	h.Done = time.Now().UnixNano()
	p.saveHealCh <- h
}

func (p *Placer) findHeal(id int64, withOrders bool) *store.Heal {
	got, ok := p.healMap.Load(id)
	if ok {
		return got.(*store.Heal)
	}
	heal, err := p.store.SelectHealByID(id)
	if err != nil {
		return nil
	}
	if withOrders {
		p.store.FindHealOrders(heal)
	}
	return heal
}

var minusOneDec = decimal.NewFromFloat(-1)

func (p *Placer) reHeal(order store.Order, clientID ClientID) {
	h := p.findHeal(clientID.ID, true)
	if h == nil {
		p.log.Error("not_found_heal", zap.Any("id", clientID.ID))
		return
	}
	for i := 0; i < len(h.Orders); i++ {
		if h.Orders[i].ID == order.ID {
			h.Orders[i] = &order
			break
		}
	}
	var reverseInc bool
	if time.Since(order.CreatedAt) > reHealPeriod {
		p.log.Info("found_heal",
			zap.Duration("since", time.Since(order.CreatedAt)),
			zap.Int("order_count", len(h.Orders)),
			zap.Any("order_id", order.ID),
			zap.Any("orders", h.Orders),
		)
		reverseInc = true
	}
	//filledSizeSum := decimal.NewFromFloat(order.FilledSize)
	var filledSizeSum decimal.Decimal
	for _, o := range h.Orders {
		filledSizeSum = filledSizeSum.Add(decimal.NewFromFloat(o.FilledSize))
	}
	h.PlaceParams.Size = h.FilledSize.Sub(filledSizeSum).Div(h.MinSize).Floor().Mul(h.MinSize)
	if h.PlaceParams.Size.LessThan(h.MinSize) {
		p.healMap.Delete(clientID.ID)
		p.log.Info("heal_filled",
			zap.Int64("i", h.ID),
			zap.String("m", h.PlaceParams.Market),
			zap.String("s", string(h.PlaceParams.Side)),
			zap.Float64("bf_size", h.FilledSize.InexactFloat64()),
			zap.Float64("hf_size", filledSizeSum.InexactFloat64()),
			//zap.Float64("size", h.PlaceParams.Size.InexactFloat64()),
			zap.Float64("min_size", h.MinSize.InexactFloat64()),
			zap.Int("h_count", len(h.Orders)),
			//zap.Any("order", order),
			zap.Int64("el", (time.Now().UnixNano()-h.ID)/million),
			zap.Int64("done_id_el", (h.Done-h.ID)/million),
			zap.Duration("since_created", time.Since(order.CreatedAt)),
		)
		return
	}

	clientID.Try = clientID.Try + 1
	h.PlaceParams.ClientID = marshalClientID(clientID)
	//TargetProfit*2 from original price
	priceInc := h.PlaceParams.Price.Div(d100).Mul(p.placeConfig.TargetProfit.Mul(d2))
	if priceInc.LessThan(h.PriceIncrement) {
		p.log.Info("price_inc_too_low",
			zap.Int64("i", h.ID),
			zap.String("m", h.PlaceParams.Market),
			zap.String("s", string(h.PlaceParams.Side)),
			zap.Float64("calc_price_inc", priceInc.InexactFloat64()),
			zap.Float64("market_price_inc", h.PriceIncrement.InexactFloat64()),
			zap.Float64("price", h.PlaceParams.Price.InexactFloat64()),
			zap.Float64("target_p", p.placeConfig.TargetProfit.InexactFloat64()),
		)
		priceInc = h.PriceIncrement
	}
	if h.PlaceParams.Side == store.SideSell {
		if reverseInc {
			priceInc = priceInc.Mul(minusOneDec)
		}
	} else {
		if !reverseInc {
			priceInc = priceInc.Mul(minusOneDec)
		}
	}
	h.PlaceParams.Price = h.PlaceParams.Price.Add(priceInc).Div(h.PriceIncrement).Floor().Mul(h.PriceIncrement)

	msg := fmt.Sprintf("heal_price_inc:%v", priceInc)
	if h.ErrorMsg != nil {
		msg = fmt.Sprintf("%s :: %s", msg, *h.ErrorMsg)
	}
	h.ErrorMsg = ftxapi.StringPointer(msg)
	p.placeHeal(h)
	p.log.Info("heal_add",
		zap.Int64("i", h.ID),
		zap.String("m", h.PlaceParams.Market),
		zap.String("s", string(h.PlaceParams.Side)),
		zap.Float64("bf_size", h.FilledSize.InexactFloat64()),
		zap.Float64("hf_size", filledSizeSum.InexactFloat64()),
		zap.Float64("size", h.PlaceParams.Size.InexactFloat64()),
		zap.Float64("min_size", h.MinSize.InexactFloat64()),
		zap.Bool("is_reverse", reverseInc),
		zap.Float64("price_inc", priceInc.InexactFloat64()),
		zap.Int("h_count", len(h.Orders)),
		zap.Int64("full_el", (h.Done-h.ID)/million),
		zap.Duration("since_created", time.Since(order.CreatedAt)),
		//zap.Any("order", order),
	)
}

func (p *Placer) heal(order store.Order, clientID ClientID) {
	got, ok := p.surebetMap.LoadAndDelete(clientID.ID)
	if !ok {
		p.log.Warn("not_found_surebet_in_map", zap.Any("order", order))
		return
	}
	lock := p.Lock(symbolFromMarket(order.Market))
	defer func() {
		id := <-lock
		p.log.Debug("unlock", zap.Int64("id", id), zap.String("m", order.Market), zap.Int64("elapsed", (time.Now().UnixNano()-id)/1000000))
	}()
	if order.FilledSize == 0 {
		p.deleteSbCh <- order.ID
		return
	}
	sb := got.(*store.Surebet)
	h := &store.Heal{
		ID:             sb.ID,
		Start:          time.Now().UnixNano(),
		FilledSize:     decimal.NewFromFloat(order.FilledSize),
		AvgFillPrice:   decimal.NewFromFloat(order.AvgFillPrice),
		MinSize:        sb.Market.MinProvideSize,
		PriceIncrement: sb.Market.PriceIncrement,
		PlaceParams: store.PlaceParamsEmb{
			Market:   sb.PlaceParams.Market,
			Type:     store.OrderTypeLimit,
			Ioc:      false,
			PostOnly: true,
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
		h.PlaceParams.Price = h.AvgFillPrice.Mul(h.FilledSize).Sub(h.FeePart).Sub(h.ProfitPart).Div(h.PlaceParams.Size)
	} else {
		h.PlaceParams.Side = store.SideSell
		h.PlaceParams.Price = h.AvgFillPrice.Mul(h.FilledSize).Add(h.FeePart).Add(h.ProfitPart).Div(h.PlaceParams.Size)
	}
	h.PlaceParams.Price = h.PlaceParams.Price.Div(sb.Market.PriceIncrement).Floor().Mul(sb.Market.PriceIncrement)

	if h.PlaceParams.Size.LessThan(h.MinSize) {
		p.log.Warn("size_too_small_to_heal", zap.Any("h", h))
		msg := fmt.Sprintf("size:%v min_provide:%v", h.PlaceParams.Size, sb.Market.MinProvideSize)
		h.ErrorMsg = ftxapi.StringPointer(msg)
		h.Done = time.Now().UnixNano()
		h.ProfitPart = decimal.Zero
		p.saveHealCh <- h
		return
	}
	p.placeHeal(h)
	p.log.Info("heal",
		zap.Int64("i", h.ID),
		zap.String("m", h.PlaceParams.Market),
		zap.String("s", string(h.PlaceParams.Side)),
		zap.Float64("pr", h.PlaceParams.Price.InexactFloat64()),
		zap.Float64("sz", h.PlaceParams.Size.InexactFloat64()),
		zap.Int64("v", h.PlaceParams.Size.Mul(h.PlaceParams.Price).IntPart()),
		zap.Any("p_part", h.ProfitPart),
		zap.Any("msg", h.ErrorMsg),
		//zap.String("c_id", h.PlaceParams.ClientID),
		zap.Int64("h_el", (h.Done-h.Start)/million),
		zap.Int64("full_el", (h.Done-h.ID)/million),
	)
}
