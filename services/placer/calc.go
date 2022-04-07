package placer

import (
	"context"
	"errors"
	"github.com/aibotsoft/crypto-surebet/pkg/store"
	ftxapi "github.com/aibotsoft/ftx-api"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"runtime"
	"strings"
	"time"
)

func (p *Placer) Calc(sb *store.Surebet) chan int64 {
	sb.StartTime = time.Now().UnixNano()
	sb.Market = p.FindMarket(sb.FtxTicker.Symbol)

	lockTimer, cancel := context.WithTimeout(p.ctx, p.cfg.Service.MaxLockTime)
	defer cancel()

	lock := p.Lock(sb.Market.BaseCurrency)
	select {
	case lock <- sb.ID:
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
		return nil
	}
	p.lastFtxPriceMap.Store(sb.FtxTicker.Symbol, sb.FtxTicker.BidPrice)
	sb.MaxStake = p.placeConfig.MaxStake
	sb.TargetProfit = p.placeConfig.TargetProfit
	sb.TargetAmount = p.placeConfig.TargetAmount
	sb.MinVolume = p.placeConfig.MinVolume

	sb.RealFee = p.accountInfo.TakerFee.Sub(p.accountInfo.TakerFee.Mul(p.placeConfig.ReferralRate)).Mul(d100)
	sb.BaseOpenBuy, sb.BaseOpenSell = p.GetOpenBuySell(sb.Market.BaseCurrency)

	sb.BaseBalance = p.FindBalance(sb.Market.BaseCurrency)
	sb.BaseTotal = sb.BaseBalance.Free.Add(sb.BaseOpenBuy).Sub(sb.BaseOpenSell)
	//sb.AmountCoef = sb.BaseBalance.UsdValue.Div(sb.MaxStake).Sub(sb.TargetAmount).Mul(sb.ProfitInc).Round(5)
	sb.ProfitInc = sb.BaseOpenBuy.Add(sb.BaseOpenSell).DivRound(sb.BaseTotal, 4)
	sb.AmountCoef = sb.ProfitInc.Mul(p.placeConfig.ProfitIncRatio).Mul(sb.TargetProfit).Round(4)

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
		sb.Size = sb.FtxTicker.AskQty
		sb.BinPrice = sb.BinTicker.BidPrice
		sb.BinSize = sb.BinTicker.BidQty
		sb.Profit = sb.BuyProfit
	} else {
		sb.PlaceParams.Side = store.SideSell
		sb.Price = sb.FtxTicker.BidPrice
		sb.Size = sb.FtxTicker.BidQty
		sb.BinPrice = sb.BinTicker.AskPrice
		sb.BinSize = sb.BinTicker.AskQty
		sb.Profit = sb.SellProfit
	}
	sb.ProfitSubSpread = sb.Profit.Sub(sb.FtxSpread)
	sb.ProfitSubFee = sb.ProfitSubSpread.Sub(sb.RealFee)

	sb.AvgPriceDiffRatio = p.placeConfig.AvgPriceDiffRatio
	if sb.PlaceParams.Side == store.SideBuy {
		sb.ProfitSubAvg = sb.ProfitSubFee.Sub(sb.AvgPriceDiff.Div(sb.AvgPriceDiffRatio)).Round(5)
	} else {
		sb.ProfitSubAvg = sb.ProfitSubFee.Add(sb.AvgPriceDiff.Div(sb.AvgPriceDiffRatio)).Round(5)
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
		return lock
	}
	if time.Duration(sb.StartTime-sb.ID) > p.cfg.Service.SendReceiveMaxDelay {
		p.log.Info("lock_time_too_high",
			zap.Int64("i", sb.ID),
			zap.String("s", sb.FtxTicker.Symbol),
			zap.Duration("start_vs_id", time.Duration(sb.StartTime-sb.ID)),
			zap.Duration("send_receive_max_delay", p.cfg.Service.SendReceiveMaxDelay),
		)
		return lock
	}

	profitDiff := sb.ProfitSubAvg.Sub(sb.RequiredProfit).Div(p.placeConfig.ProfitDiffRatio)
	sb.ProfitPriceDiff = sb.Price.Mul(profitDiff).DivRound(d100, 6)

	sb.BinVolume = sb.BinPrice.Mul(sb.BinSize).Floor()
	var maxSizeByFree decimal.Decimal
	if sb.PlaceParams.Side == store.SideSell {
		sb.PlaceParams.Price = sb.Price.Sub(sb.ProfitPriceDiff).Div(sb.Market.PriceIncrement).Floor().Mul(sb.Market.PriceIncrement)
		maxSizeByFree = sb.BaseBalance.Free
	} else {
		sb.PlaceParams.Price = sb.Price.Add(sb.ProfitPriceDiff).Div(sb.Market.PriceIncrement).Floor().Mul(sb.Market.PriceIncrement)
		maxSizeByFree = sb.QuoteBalance.Free.Div(sb.PlaceParams.Price)
	}
	maxSizeByTotal := sb.BaseTotal.Div(sb.TargetAmount)
	maxSizeByBinSize := sb.BinSize.Div(p.placeConfig.BinFtxVolumeRatio)
	maxSizeByMaxStake := sb.MaxStake.Div(sb.PlaceParams.Price)
	sb.SizeRatio = sb.Size.Div(sb.BinSize).Add(d1)
	size := decimal.Min(
		maxSizeByTotal,
		maxSizeByMaxStake,
		maxSizeByFree,
		maxSizeByBinSize,
	)
	if size.Equal(maxSizeByTotal) {
		sb.MaxBy = "total"
	} else if size.Equal(maxSizeByMaxStake) {
		sb.MaxBy = "max_stake"
	} else if size.Equal(maxSizeByFree) {
		sb.MaxBy = "free"
	} else if size.Equal(maxSizeByBinSize) {
		sb.MaxBy = "bin_size"
	}
	sb.PlaceParams.Size = size.Div(sb.Market.MinProvideSize).Floor().Mul(sb.Market.MinProvideSize)

	sb.Volume = sb.PlaceParams.Size.Mul(sb.PlaceParams.Price).Floor()
	if sb.Volume.LessThan(sb.MinVolume) {
		p.log.Info("vol_low",
			zap.String("by", sb.MaxBy),
			//zap.Float64("", sb.MaxBy),

			zap.String("m", sb.FtxTicker.Symbol),
			zap.String("s", string(sb.PlaceParams.Side)),
			zap.Float64("pr", sb.PlaceParams.Price.InexactFloat64()),
			zap.Float64("sz", sb.PlaceParams.Size.InexactFloat64()),
			zap.Int64("v", sb.Volume.IntPart()),
			zap.Int64("min_v", sb.MinVolume.IntPart()),
			zap.Float64("min_size", sb.Market.MinProvideSize.InexactFloat64()),
			zap.Float64("b_free", sb.BaseBalance.Free.InexactFloat64()),
			zap.Int64("q_free", sb.QuoteBalance.Free.IntPart()),
			zap.Int64("vol_by_bin", sb.BinVolume.Div(p.placeConfig.BinFtxVolumeRatio).IntPart()),
		)
		p.checkBalanceCh <- sb.Done
		return lock
	}

	sb.MakerFee = p.accountInfo.MakerFee
	sb.TakerFee = p.accountInfo.TakerFee
	sb.PlaceParams.Market = sb.FtxTicker.Symbol
	sb.PlaceParams.Type = store.OrderTypeLimit
	sb.PlaceParams.Ioc = true
	sb.PlaceParams.PostOnly = false
	sb.PlaceParams.ClientID = marshalClientID(ClientID{ID: sb.ID, Side: BET})
	sb.BeginPlace = time.Now().UnixNano()

	if p.cfg.Service.DemoMode {
		p.log.Info("demo_mode",
			//zap.Int64("id", sb.ID),
			zap.String("m", sb.PlaceParams.Market),
			zap.String("s", string(sb.PlaceParams.Side)),
			//zap.Any("clientID", sb.PlaceParams.ClientID),
			zap.Float64("pr", sb.Price.InexactFloat64()),
			zap.Float64("place_p", sb.PlaceParams.Price.InexactFloat64()),
			zap.Float64("sz", sb.PlaceParams.Size.InexactFloat64()),
			//zap.Any("profit_inc", sb.ProfitInc),
			zap.Float64("a_coef", sb.AmountCoef.InexactFloat64()),
			//zap.Any("target_amount", sb.TargetAmount),
			//zap.Any("target_p", sb.TargetProfit),
			zap.Float64("req_p", sb.RequiredProfit.InexactFloat64()),
			//zap.Any("profit", sb.Profit),
			//zap.Any("profit_sub_spread", sb.ProfitSubSpread),
			zap.Float64("p_sub_fee", sb.ProfitSubFee.InexactFloat64()),
			zap.Float64("p_sub_avg", sb.ProfitSubAvg.InexactFloat64()),
			zap.Float64("profit_price_diff", sb.ProfitPriceDiff.InexactFloat64()),
			//zap.Any("real_fee", sb.RealFee),
			zap.Float64("total", sb.BaseTotal.InexactFloat64()),
			zap.Float64("open_buy", sb.BaseOpenBuy.InexactFloat64()),
			zap.Float64("open_sell", sb.BaseOpenSell.InexactFloat64()),
			zap.Float64("avg_price_diff", sb.AvgPriceDiff.InexactFloat64()),
			zap.Float64("ftx_spread", sb.FtxSpread.InexactFloat64()),
			zap.Float64("bin_spread", sb.BinSpread.InexactFloat64()),
		)
		time.Sleep(time.Millisecond * 50)
		//p.saveSbCh <- sb
		//p.checkBalanceCh <- time.Now().UnixNano()
		return lock
	}

	p.surebetMap.Store(sb.ID, sb)
	order, err := p.PlaceOrder(p.ctx, sb.PlaceParams)
	sb.Done = time.Now().UnixNano()
	if err != nil {
		if errors.Is(err, ftxapi.ErrorRateLimit) {
			p.log.Warn("bet_error",
				zap.Error(err),
				zap.String("s", sb.FtxTicker.Symbol),
				zap.Duration("elapsed", time.Duration(sb.Done-sb.StartTime)))
		} else {
			p.log.Warn("bet_error", zap.Error(err), zap.Any("sb", sb), zap.Duration("elapsed", time.Duration(sb.Done-sb.StartTime)))
		}
		return lock
	}
	sb.OrderID = order.ID
	p.saveSbCh <- sb

	p.log.Info("bet",
		zap.Int64("i", sb.ID),
		zap.String("m", sb.PlaceParams.Market),
		zap.String("s", string(sb.PlaceParams.Side)),
		zap.Float64("pr", sb.PlaceParams.Price.InexactFloat64()),
		zap.Float64("sz", sb.PlaceParams.Size.InexactFloat64()),

		zap.Float64("size", sb.Size.InexactFloat64()),
		zap.Float64("bsize", sb.BinSize.InexactFloat64()),

		zap.Int64("v", sb.Volume.IntPart()),
		zap.Int64("bv", sb.BinVolume.IntPart()),
		//zap.Any("target_amount", sb.TargetAmount),
		//zap.Any("target_p", sb.TargetProfit),
		//zap.Any("profit_inc", sb.ProfitInc),
		zap.Float64("a_coef", sb.AmountCoef.InexactFloat64()),
		zap.Float64("avg_price_diff", sb.AvgPriceDiff.InexactFloat64()),

		//zap.Any("prof", sb.Profit),
		//zap.Float64("p_sub_fee", sb.ProfitSubFee.InexactFloat64()),
		//zap.Any("profit_sub_spread", sb.ProfitSubSpread),
		zap.Float64("p_sub_avg", sb.ProfitSubAvg.InexactFloat64()),
		zap.Float64("req_p", sb.RequiredProfit.InexactFloat64()),

		//zap.Any("real_fee", sb.RealFee),
		//zap.Any("p_price_diff", sb.ProfitPriceDiff),
		zap.Float64("base_total", sb.BaseTotal.InexactFloat64()),
		zap.Float64("o_buy", sb.BaseOpenBuy.InexactFloat64()),
		zap.Float64("o_sell", sb.BaseOpenSell.InexactFloat64()),
		zap.Float64("ftx_sp", sb.FtxSpread.InexactFloat64()),
		zap.Float64("bin_sp", sb.BinSpread.InexactFloat64()),
		zap.String("by", sb.MaxBy),
		zap.Float64("sr", sb.SizeRatio.InexactFloat64()),

		//zap.Int64("place_count", placeCounter.Load()),
		//zap.Int64("fills_count", fillsCounter.Load()),
		zap.Int64("el", time.Duration(sb.Done-sb.BeginPlace).Milliseconds()),
	)
	if order != nil {
		if sb.PlaceParams.Side == store.SideSell {
			p.BalanceAdd(sb.Market.QuoteCurrency, sb.Volume, sb.Volume)
			p.BalanceSub(sb.Market.BaseCurrency, sb.Volume, sb.PlaceParams.Size)
		} else {
			p.BalanceSub(sb.Market.QuoteCurrency, sb.Volume, sb.Volume)
			p.BalanceAdd(sb.Market.BaseCurrency, sb.Volume, sb.PlaceParams.Size)
		}
	}
	p.checkBalanceCh <- sb.Done
	return nil
}

//if time.Duration(sb.BinTicker.ReceiveTime-sb.FtxTicker.ReceiveTime) < -p.cfg.Service.BinanceMaxDelay {
//	p.log.Info("binance_delayed",
//		zap.String("s", sb.FtxTicker.Symbol),
//		zap.Int64("bin_ftx_diff", time.Duration(sb.BinTicker.ReceiveTime-sb.FtxTicker.ReceiveTime).Milliseconds()),
//		zap.Duration("binance_max_delay", -p.cfg.Service.BinanceMaxDelay),
//		zap.Duration("last_bin_time_to_now", time.Duration(sb.StartTime-sb.LastBinTime)),
//		zap.Duration("ftx_receive_vs_server", time.Duration(sb.FtxTicker.ReceiveTime-sb.FtxTicker.ServerTime)),
//		zap.Duration("start_vs_id", time.Duration(sb.StartTime-sb.ID)),
//		zap.Any("clear_p", sb.ProfitSubAvg),
//	)
//	return lock
//}
//if sb.ID != sb.BinTicker.ReceiveTime && time.Duration(sb.StartTime-sb.LastBinTime) > p.cfg.Service.BinanceMaxStaleTime {
//	p.log.Info("binance_stale",
//		zap.String("symbol", sb.FtxTicker.Symbol),
//		zap.Duration("last_bin_time_to_now", time.Duration(sb.StartTime-sb.LastBinTime)),
//		zap.Duration("binance_max_stale_time", p.cfg.Service.BinanceMaxStaleTime),
//		zap.Duration("ftx_st_vs_rt", time.Duration(sb.FtxTicker.ReceiveTime-sb.FtxTicker.ServerTime)))
//	return lock
//}
//if sb.PlaceParams.Size.LessThan(sb.Market.MinProvideSize) {
//p.log.Info("stake_low",
//zap.String("m", sb.FtxTicker.Symbol),
//zap.String("s", string(sb.PlaceParams.Side)),
//zap.Float64("sz", sb.PlaceParams.Size.InexactFloat64()),
//zap.Float64("min_size", sb.Market.MinProvideSize.InexactFloat64()),
//zap.Float64("b_free", sb.BaseBalance.Free.InexactFloat64()),
//zap.Int64("q_free", sb.QuoteBalance.Free.IntPart()),
//zap.Int64("bin_vol", sb.BinVolume.IntPart()),
//zap.Float64("clear_p", sb.ProfitSubAvg.InexactFloat64()),
//zap.Float64("req_p", sb.RequiredProfit.InexactFloat64()),
//zap.Float64("avg_price_diff", sb.AvgPriceDiff.InexactFloat64()),
//zap.Float64("a_coef", sb.AmountCoef.InexactFloat64()),
//)
//return lock
//}
