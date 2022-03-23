package placer

import (
	"context"
	"fmt"
	"github.com/aibotsoft/crypto-surebet/pkg/store"
	"github.com/jinzhu/copier"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"runtime"
	"strings"
	"time"
)

func (p *Placer) Calc(sb *store.Surebet) {
	sb.StartTime = time.Now().UnixNano()
	sb.Market = p.FindMarket(sb.FtxTicker.Symbol)

	lockTimer, cancel := context.WithTimeout(p.ctx, p.cfg.Service.MaxLockTime)
	defer cancel()

	lock := p.Lock(sb.Market.BaseCurrency)
	select {
	case lock <- true:
		//p.log.Info("got_lock",
		//	zap.String("s", sb.Market.BaseCurrency),
		//	zap.Int64("id", sb.ID),
		//	zap.Duration("lock_elapsed", time.Duration(time.Now().UnixNano()-sb.StartTime)),
		//	zap.Duration("max_lock_time", p.cfg.Service.MaxLockTime),
		//	zap.Int("goroutine", runtime.NumGoroutine()),
		//)
	case <-lockTimer.Done():
		p.log.Debug("lock_too_long",
			zap.String("s", sb.Market.BaseCurrency),
			zap.Int64("id", sb.ID),
			zap.Duration("lock_elapsed", time.Duration(time.Now().UnixNano()-sb.StartTime)),
			zap.Duration("max_lock_time", p.cfg.Service.MaxLockTime),
			zap.Int("goroutine", runtime.NumGoroutine()),
		)
		return
	}
	defer func() {
		<-lock
	}()
	sb.MaxStake = p.placeConfig.MaxStake
	sb.TargetProfit = p.placeConfig.TargetProfit
	sb.TargetAmount = p.placeConfig.TargetAmount

	sb.RealFee = p.accountInfo.TakerFee.Sub(p.accountInfo.TakerFee.Mul(p.placeConfig.ReferralRate)).Mul(d100)
	sb.BaseOpenBuy, sb.BaseOpenSell = p.GetOpenBuySell(sb.Market.BaseCurrency)

	sb.BaseBalance = p.FindBalance(sb.Market.BaseCurrency)
	sb.BaseTotal = sb.BaseBalance.Free.Add(sb.BaseOpenBuy).Sub(sb.BaseOpenSell)
	//sb.AmountCoef = sb.BaseBalance.UsdValue.Div(sb.MaxStake).Sub(sb.TargetAmount).Mul(sb.ProfitInc).Round(5)
	sb.ProfitInc = sb.BaseOpenBuy.Add(sb.BaseOpenSell).DivRound(sb.BaseTotal, 4)
	sb.AmountCoef = sb.ProfitInc.Mul(d2).Mul(sb.TargetProfit).Round(4)

	sb.QuoteBalance = p.FindBalance(sb.Market.QuoteCurrency)

	sb.FtxSpread = sb.FtxTicker.AskPrice.Sub(sb.FtxTicker.BidPrice).Mul(d100).DivRound(sb.FtxTicker.AskPrice, 6)
	sb.BinSpread = sb.BinTicker.AskPrice.Sub(sb.BinTicker.BidPrice).Mul(d100).DivRound(sb.BinTicker.AskPrice, 6)
	if strings.Index(sb.FtxTicker.Symbol, usdt) == -1 {
		sb.BuyProfit = sb.BinTicker.BidPrice.Sub(sb.FtxTicker.AskPrice.Div(sb.UsdtPrice)).Mul(d100).DivRound(sb.BinTicker.BidPrice, 6)
		sb.SellProfit = sb.FtxTicker.BidPrice.Div(sb.UsdtPrice).Sub(sb.BinTicker.AskPrice).Mul(d100).DivRound(sb.FtxTicker.BidPrice.Div(sb.UsdtPrice), 6)
	} else {
		sb.BuyProfit = sb.BinTicker.BidPrice.Sub(sb.FtxTicker.AskPrice).Mul(d100).DivRound(sb.BinTicker.BidPrice, 6)
		sb.SellProfit = sb.FtxTicker.BidPrice.Sub(sb.BinTicker.AskPrice).Mul(d100).DivRound(sb.FtxTicker.BidPrice, 6)
	}
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	sb.RequiredProfit = sb.TargetProfit.Add(sb.AmountCoef)
	if sb.BuyProfit.GreaterThan(sb.SellProfit) {
		sb.PlaceParams.Side = store.SideBuy
		sb.Price = sb.FtxTicker.AskPrice
		sb.BinPrice = sb.BinTicker.BidPrice
		sb.Profit = sb.BuyProfit
	} else {
		sb.PlaceParams.Side = store.SideSell
		sb.Price = sb.FtxTicker.BidPrice
		sb.BinPrice = sb.BinTicker.AskPrice
		sb.Profit = sb.SellProfit
	}
	sb.ProfitSubSpread = sb.Profit.Sub(sb.FtxSpread)
	sb.ProfitSubFee = sb.ProfitSubSpread.Sub(sb.RealFee)

	if sb.PlaceParams.Side == store.SideBuy {
		sb.ProfitSubAvg = sb.ProfitSubFee.Sub(sb.AvgPriceDiff.Div(d10))
	} else {
		sb.ProfitSubAvg = sb.ProfitSubFee.Add(sb.AvgPriceDiff.Div(d10))
	}
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	if sb.ProfitSubAvg.LessThan(sb.RequiredProfit) {
		//p.log.Debug("profit_too_low",
		//	zap.String("symbol", sb.FtxTicker.Symbol),
		//	//zap.Any("QuoteBalance", sb.QuoteBalance),
		//	zap.Any("side", sb.PlaceParams.Side),
		//	zap.Any("profit_sub_fee", sb.ProfitSubFee),
		//	zap.Any("profit_sub_spread", sb.ProfitSubSpread),
		//	zap.Any("profit", sb.Profit),
		//	zap.Any("AmountCoef", sb.AmountCoef),
		//	zap.Any("free", sb.BaseBalance.Free),
		//	zap.Any("buy", sb.BaseOpenBuy),
		//	zap.Any("sell", sb.BaseOpenSell),
		//	zap.Any("total", sb.BaseTotal),
		//	//zap.Any("sb.AvgPriceDiff", sb.AvgPriceDiff),
		//	zap.Any("required_profit", sb.RequiredProfit),
		//	//zap.Any("target_profit", sb.TargetProfit),
		//	//zap.Any("amount_coef", sb.AmountCoef),
		//	//zap.Any("real_fee", sb.RealFee),
		//	zap.Any("profit_inc", sb.ProfitInc),
		//)
		return
	}
	if time.Duration(sb.StartTime-sb.ID) > p.cfg.Service.SendReceiveMaxDelay {
		p.log.Info("lock_time_too_high", zap.String("symbol", sb.FtxTicker.Symbol), zap.Duration("start_vs_id", time.Duration(sb.StartTime-sb.ID)))
		return
	}
	if sb.ID != sb.BinTicker.ReceiveTime && time.Duration(sb.StartTime-sb.LastBinTime) > p.cfg.Service.BinanceMaxStaleTime {
		p.log.Info("binance_stale", zap.String("symbol", sb.FtxTicker.Symbol), zap.Duration("last_bin_time_to_now", time.Duration(sb.StartTime-sb.LastBinTime)), zap.Duration("ftx_st_vs_rt", time.Duration(sb.FtxTicker.ReceiveTime-sb.FtxTicker.ServerTime)))
		return
	}
	profitDiff := sb.ProfitSubAvg.Sub(sb.RequiredProfit).Div(d2)
	sb.ProfitPriceDiff = sb.Price.Mul(profitDiff).DivRound(d100, 6)

	maxSize := sb.BaseTotal.Div(sb.TargetAmount)

	var size decimal.Decimal
	if sb.PlaceParams.Side == store.SideSell {
		sb.PlaceParams.Price = sb.Price.Sub(sb.ProfitPriceDiff).Div(sb.Market.PriceIncrement).Floor().Mul(sb.Market.PriceIncrement)
		sb.BinVolume = sb.BinPrice.Mul(sb.BinTicker.AskQty)
		size = decimal.Min(
			maxSize,
			sb.MaxStake.Div(sb.PlaceParams.Price),
			sb.BaseBalance.Free,
			sb.BinTicker.AskQty.Div(p.placeConfig.BinFtxVolumeRatio),
		)
	} else {
		sb.PlaceParams.Price = sb.Price.Add(sb.ProfitPriceDiff).Div(sb.Market.PriceIncrement).Floor().Mul(sb.Market.PriceIncrement)
		sb.BinVolume = sb.BinPrice.Mul(sb.BinTicker.BidQty)
		size = decimal.Min(
			maxSize,
			sb.MaxStake.Div(sb.PlaceParams.Price),
			sb.QuoteBalance.Free.Div(sb.PlaceParams.Price),
			sb.BinTicker.BidQty.Div(p.placeConfig.BinFtxVolumeRatio),
		)
	}
	if size.LessThan(sb.Market.MinProvideSize) {
		p.log.Info("stake_too_low",
			zap.String("symbol", sb.FtxTicker.Symbol),
			zap.Any("side", sb.PlaceParams.Side),
			zap.Any("size", size),
			zap.Any("min_provide", sb.Market.MinProvideSize),
			zap.Int64("base_usd_value", sb.BaseBalance.UsdValue.IntPart()),
			zap.Int64("quote_free", sb.QuoteBalance.Free.IntPart()),
			zap.Any("bin_volume", sb.BinVolume),
			zap.Any("base_free", sb.BaseBalance.Free),
			zap.Any("required_profit", sb.RequiredProfit),
			zap.Any("profit_sub_avg", sb.ProfitSubAvg),
			zap.Duration("elapsed", time.Duration(time.Now().UnixNano()-sb.StartTime)),
			//zap.Int("goroutine", runtime.NumGoroutine()),
		)
		return
	}
	if time.Duration(sb.BinTicker.ReceiveTime-sb.FtxTicker.ReceiveTime) < -p.cfg.Service.BinanceMaxDelay {
		p.log.Info("binance_too_delayed",
			zap.String("symbol", sb.FtxTicker.Symbol),
			zap.Duration("bin_ftx_diff", time.Duration(sb.BinTicker.ReceiveTime-sb.FtxTicker.ReceiveTime)),
			zap.Duration("binance_max_delay", -p.cfg.Service.BinanceMaxDelay),
			zap.Duration("last_bin_time_to_now", time.Duration(sb.StartTime-sb.LastBinTime)),
			zap.Duration("ftx_st_vs_rt", time.Duration(sb.FtxTicker.ReceiveTime-sb.FtxTicker.ServerTime)),
			zap.Duration("start_vs_id", time.Duration(sb.StartTime-sb.ID)),
		)
		return
	}
	sb.MakerFee = p.accountInfo.MakerFee
	sb.TakerFee = p.accountInfo.TakerFee
	sb.PlaceParams.Size = size.Div(sb.Market.MinProvideSize).Floor().Mul(sb.Market.MinProvideSize)
	sb.Volume = sb.PlaceParams.Size.Mul(sb.PlaceParams.Price).Round(5)
	sb.PlaceParams.Market = sb.FtxTicker.Symbol
	sb.PlaceParams.Type = store.OrderTypeLimit
	sb.PlaceParams.Ioc = true
	sb.PlaceParams.PostOnly = false
	sb.PlaceParams.ClientID = fmt.Sprintf("%d:%s", sb.ID, BET)

	//lockID, gotLock := p.Lock(sb.Market.BaseCurrency, sb.ID)
	//if !gotLock {
	//	p.log.Debug("not_got_lock",
	//		zap.String("symbol", sb.FtxTicker.Symbol),
	//		zap.Any("id", sb.ID),
	//		zap.Any("lockID", lockID),
	//		zap.Int("goroutine", runtime.NumGoroutine()),
	//	)
	//	return
	//}
	//defer p.Unlock(sb.Market.BaseCurrency)

	sb.BeginPlace = time.Now().UnixNano()

	if p.cfg.Service.DemoMode {
		p.log.Info("demo_mode",
			//zap.Any("id", sb.ID),
			zap.Any("m", sb.PlaceParams.Market),
			zap.Any("s", sb.PlaceParams.Side),
			zap.Any("price", sb.Price),
			zap.Any("place_price", sb.PlaceParams.Price),
			zap.Any("size", sb.PlaceParams.Size),
			//zap.Any("profit_inc", sb.ProfitInc),
			zap.Any("amount_coef", sb.AmountCoef),
			//zap.Any("target_amount", sb.TargetAmount),
			//zap.Any("target_p", sb.TargetProfit),
			zap.Any("req_p", sb.RequiredProfit),
			//zap.Any("profit", sb.Profit),
			//zap.Any("profit_sub_spread", sb.ProfitSubSpread),
			zap.Any("p_sub_fee", sb.ProfitSubFee),
			zap.Any("p_sub_avg", sb.ProfitSubAvg),
			zap.Any("profit_price_diff", sb.ProfitPriceDiff),
			//zap.Any("real_fee", sb.RealFee),
			zap.Any("total", sb.BaseTotal),
			zap.Any("open_buy", sb.BaseOpenBuy),
			zap.Any("open_sell", sb.BaseOpenSell),
			zap.Any("avg_price_diff", sb.AvgPriceDiff),
			zap.Any("ftx_spread", sb.FtxSpread),
			zap.Any("bin_spread", sb.BinSpread),
		)
		return
	}

	p.surebetMap.Store(sb.PlaceParams.ClientID, sb)
	order, err := p.PlaceOrder(p.ctx, sb.PlaceParams)
	if err != nil {
		p.log.Warn("place_error", zap.Error(err), zap.Any("sb", sb), zap.Duration("elapsed", time.Duration(time.Now().UnixNano()-sb.StartTime)))
		return
	}
	sb.Done = time.Now().UnixNano()
	sb.OrderID = order.ID
	p.saveSbCh <- sb
	var o store.Order
	_ = copier.Copy(&o, order)
	p.orderMap.Store(o.ID, o)

	placeCounter.Inc()
	p.log.Info("bet",
		zap.Any("id", sb.ID),
		zap.Any("m", sb.PlaceParams.Market),
		zap.Any("s", sb.PlaceParams.Side),
		zap.Any("price", sb.PlaceParams.Price),
		zap.Any("size", sb.PlaceParams.Size),
		//zap.Any("target_amount", sb.TargetAmount),
		zap.Any("target_p", sb.TargetProfit),
		zap.Any("req_p", sb.RequiredProfit),
		//zap.Any("profit_inc", sb.ProfitInc),
		zap.Any("a_coef", sb.AmountCoef),
		zap.Any("prof", sb.Profit),
		zap.Any("p_sub_fee", sb.ProfitSubFee),
		//zap.Any("profit_sub_spread", sb.ProfitSubSpread),
		zap.Any("p_sub_avg", sb.ProfitSubAvg),
		zap.Any("avg_price_diff", sb.AvgPriceDiff),
		zap.Any("real_fee", sb.RealFee),
		//zap.Any("p_price_diff", sb.ProfitPriceDiff),
		zap.Any("base_total", sb.BaseTotal),
		zap.Any("open_buy", sb.BaseOpenBuy),
		zap.Any("open_sell", sb.BaseOpenSell),
		zap.Any("ftx_spread", sb.FtxSpread),
		zap.Any("bin_spread", sb.BinSpread),
		zap.Int64("place_count", placeCounter.Load()),
		zap.Int64("fills_count", fillsCounter.Load()),
		zap.Duration("elapsed", time.Duration(sb.Done-sb.BeginPlace)),
	)
	//p.log.Info("success",
	//	zap.Any("sb", sb),
	//	zap.Int64("place_count", placeCounter.Load()),
	//	zap.Int64("fills_count", fillsCounter.Load()),
	//	zap.Duration("place_elapsed", time.Duration(sb.Done-sb.BeginPlace)),
	//)
	if order != nil {
		if sb.PlaceParams.Side == store.SideSell {
			p.BalanceAdd(sb.Market.QuoteCurrency, sb.Volume, sb.Volume)
			p.BalanceSub(sb.Market.BaseCurrency, sb.Volume, sb.PlaceParams.Size)
		} else {
			p.BalanceSub(sb.Market.QuoteCurrency, sb.Volume, sb.Volume)
			p.BalanceAdd(sb.Market.BaseCurrency, sb.Volume, sb.PlaceParams.Size)
		}
	}
	p.checkBalanceCh <- true
}
