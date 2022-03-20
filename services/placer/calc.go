package placer

import (
	"fmt"
	"github.com/aibotsoft/crypto-surebet/pkg/store"
	"github.com/shopspring/decimal"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"runtime"
	"strings"
	"time"
)

var placeCounter, fillsCounter atomic.Int64

const usdt = "USDT"

//var UsdtPrice = decimal.RequireFromString("1.0005")

func (p *Placer) Calc(sb *store.Surebet) {
	sb.StartTime = time.Now().UnixNano()
	sb.Market = p.FindMarket(sb.FtxTicker.Symbol)
	sb.MaxStake = p.placeConfig.MaxStake
	sb.TargetProfit = p.placeConfig.TargetProfit

	sb.TargetAmount = p.targetAmount
	sb.ProfitInc = sb.TargetProfit.DivRound(sb.TargetAmount, 7)

	sb.RealFee = p.accountInfo.TakerFee.Sub(p.accountInfo.TakerFee.Mul(p.placeConfig.ReferralRate)).Mul(d100)

	sb.BaseBalance = p.FindBalance(sb.Market.BaseCurrency)
	sb.QuoteBalance = p.FindBalance(sb.Market.QuoteCurrency)
	sb.AmountCoef = sb.BaseBalance.UsdValue.Div(sb.MaxStake).Sub(sb.TargetAmount).Mul(sb.ProfitInc).Round(5)

	sb.FtxSpread = sb.FtxTicker.AskPrice.Sub(sb.FtxTicker.BidPrice).Mul(d100).DivRound(sb.FtxTicker.AskPrice, 6)
	sb.BinSpread = sb.BinTicker.AskPrice.Sub(sb.BinTicker.BidPrice).Mul(d100).DivRound(sb.BinTicker.AskPrice, 6)

	//usdUsdtPrice:=1.0005
	if strings.Index(sb.FtxTicker.Symbol, usdt) == -1 {
		sb.BuyProfit = sb.BinTicker.BidPrice.Sub(sb.FtxTicker.AskPrice.Div(sb.UsdtPrice)).Mul(d100).DivRound(sb.BinTicker.BidPrice, 6)
		//sb.BuyProfit = sb.BinTicker.BidPrice.Sub(sb.FtxTicker.AskPrice).Mul(d100).DivRound(sb.BinTicker.BidPrice, 6)
		sb.SellProfit = sb.FtxTicker.BidPrice.Div(sb.UsdtPrice).Sub(sb.BinTicker.AskPrice).Mul(d100).DivRound(sb.FtxTicker.BidPrice.Div(sb.UsdtPrice), 6)
		//sb.SellProfit = sb.FtxTicker.BidPrice.Sub(sb.BinTicker.AskPrice).Mul(d100).DivRound(sb.FtxTicker.BidPrice, 6)
		//p.log.Info("buy",
		//	//zap.Any("BuyProfit", BuyProfit),
		//	zap.Any("sb.BuyProfit", sb.BuyProfit),
		//
		//	//zap.Any("SellProfit", SellProfit),
		//	zap.Any("sb.SellProfit", sb.SellProfit),
		//)
	} else {
		sb.BuyProfit = sb.BinTicker.BidPrice.Sub(sb.FtxTicker.AskPrice).Mul(d100).DivRound(sb.BinTicker.BidPrice, 6)
		sb.SellProfit = sb.FtxTicker.BidPrice.Sub(sb.BinTicker.AskPrice).Mul(d100).DivRound(sb.FtxTicker.BidPrice, 6)
	}

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	if sb.BuyProfit.GreaterThan(sb.SellProfit) {
		sb.PlaceParams.Side = store.SideBuy
		sb.Price = sb.FtxTicker.AskPrice
		sb.Profit = sb.BuyProfit
		sb.RequiredProfit = sb.TargetProfit.Add(sb.AmountCoef)
	} else {
		sb.PlaceParams.Side = store.SideSell
		sb.Price = sb.FtxTicker.BidPrice
		sb.Profit = sb.SellProfit
		sb.RequiredProfit = decimal.Max(sb.TargetProfit.Sub(sb.AmountCoef), decimal.Zero)
	}
	sb.ProfitSubSpread = sb.Profit.Sub(sb.FtxSpread)
	sb.ProfitSubFee = sb.ProfitSubSpread.Sub(sb.RealFee)
	sb.ProfitSubAvg = sb.ProfitSubFee.Add(sb.AvgPriceDiff)
	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	if sb.ProfitSubFee.LessThan(sb.RequiredProfit) {
		//p.log.Debug("profit_too_low",
		//	zap.String("symbol", sb.FtxTicker.Symbol),
		//	//zap.Any("QuoteBalance", sb.QuoteBalance),
		//	zap.Any("side", sb.PlaceParams.Side),
		//	zap.Any("profit_sub_fee", sb.ProfitSubFee),
		//	zap.Any("profit_sub_spread", sb.ProfitSubSpread),
		//	zap.Any("profit", sb.Profit),
		//	//zap.Any("required_profit", sb.RequiredProfit),
		//	//zap.Any("target_profit", sb.TargetProfit),
		//	//zap.Any("amount_coef", sb.AmountCoef),
		//	//zap.Any("real_fee", sb.RealFee),
		//	//zap.Any("profit_inc", sb.ProfitInc),
		//)
		return
	}
	if time.Duration(sb.StartTime-sb.ID) > p.cfg.Service.SendReceiveMaxDelay {
		p.log.Debug("lock_time_too_high", zap.String("symbol", sb.FtxTicker.Symbol), zap.Duration("start_vs_id", time.Duration(sb.StartTime-sb.ID)))
		return
	}
	if sb.ID != sb.BinTicker.ReceiveTime && time.Duration(sb.StartTime-sb.LastBinTime) > p.cfg.Service.BinanceMaxStaleTime {
		p.log.Debug("binance_stale", zap.String("symbol", sb.FtxTicker.Symbol), zap.Duration("last_bin_time_to_now", time.Duration(sb.StartTime-sb.LastBinTime)), zap.Duration("ftx_st_vs_rt", time.Duration(sb.FtxTicker.ReceiveTime-sb.FtxTicker.ServerTime)))
		return
	}
	profitDiff := sb.ProfitSubFee.Sub(sb.RequiredProfit)
	sb.ProfitPriceDiff = sb.Price.Mul(profitDiff).Div(d100).Round(6)

	var size decimal.Decimal
	if sb.PlaceParams.Side == store.SideSell {
		sb.PlaceParams.Price = sb.Price.Sub(sb.ProfitPriceDiff).Div(sb.Market.PriceIncrement).Floor().Mul(sb.Market.PriceIncrement)
		sb.BinPrice = sb.BinTicker.AskPrice
		sb.BinVolume = sb.BinPrice.Mul(sb.BinTicker.AskQty)
		size = decimal.Min(sb.MaxStake.Div(sb.PlaceParams.Price), sb.BaseBalance.Free, sb.BinTicker.AskQty.Div(p.placeConfig.BinFtxVolumeRatio))
	} else {
		sb.PlaceParams.Price = sb.Price.Add(sb.ProfitPriceDiff).Div(sb.Market.PriceIncrement).Floor().Mul(sb.Market.PriceIncrement)
		sb.BinPrice = sb.BinTicker.BidPrice
		sb.BinVolume = sb.BinPrice.Mul(sb.BinTicker.BidQty)
		volume := decimal.Min(sb.MaxStake, sb.QuoteBalance.Free, sb.BinVolume.Div(p.placeConfig.BinFtxVolumeRatio))
		size = volume.Div(sb.PlaceParams.Price)
	}
	if size.LessThan(sb.Market.MinProvideSize) {
		p.log.Debug("stake_too_low",
			zap.String("symbol", sb.FtxTicker.Symbol),
			zap.Any("side", sb.PlaceParams.Side),
			zap.Any("size", size),
			zap.Any("min_provide", sb.Market.MinProvideSize),
			zap.Int64("base_usd_value", sb.BaseBalance.UsdValue.IntPart()),
			zap.Int64("quote_free", sb.QuoteBalance.Free.IntPart()),
			zap.Any("bin_volume", sb.BinVolume),
			zap.Any("base_free", sb.BaseBalance.Free),
			zap.Any("required_profit", sb.RequiredProfit),
			zap.Any("profit_sub_fee", sb.ProfitSubFee),
			//zap.Duration("elapsed", time.Duration(time.Now().UnixNano()-sb.StartTime)),
			//zap.Int("goroutine", runtime.NumGoroutine()),
		)
		return
	}
	if time.Duration(sb.BinTicker.ReceiveTime-sb.FtxTicker.ReceiveTime) < -p.cfg.Service.BinanceMaxDelay {
		p.log.Debug("binance_too_delayed",
			zap.String("symbol", sb.FtxTicker.Symbol),
			zap.Duration("bin_ftx_diff", time.Duration(sb.BinTicker.ReceiveTime-sb.FtxTicker.ReceiveTime)),
			zap.Duration("binance_max_delay", -p.cfg.Service.BinanceMaxDelay),
			zap.Duration("last_bin_time_to_now", time.Duration(sb.StartTime-sb.LastBinTime)),
			zap.Duration("ftx_st_vs_rt", time.Duration(sb.FtxTicker.ReceiveTime-sb.FtxTicker.ServerTime)),
			zap.Duration("start_vs_id", time.Duration(sb.StartTime-sb.ID)),
			zap.Int("goroutine", runtime.NumGoroutine()),
		)
		return
	}
	sb.MakerFee = p.accountInfo.MakerFee
	sb.TakerFee = p.accountInfo.TakerFee
	sb.PlaceParams.Size = size.Div(sb.Market.MinProvideSize).Floor().Mul(sb.Market.MinProvideSize)
	sb.Volume = sb.PlaceParams.Size.Mul(sb.PlaceParams.Price).Round(5)

	gotLock := p.Lock(sb.FtxTicker.Symbol)
	if !gotLock {
		//p.log.Debug("not_got_lock", zap.String("symbol", sb.FtxTicker.Symbol), zap.Int("goroutine", runtime.NumGoroutine()))
		return
	}
	defer p.Unlock(sb.FtxTicker.Symbol)

	sb.PlaceParams.Market = sb.FtxTicker.Symbol
	sb.PlaceParams.Type = store.OrderTypeLimit
	sb.PlaceParams.Ioc = true
	sb.PlaceParams.PostOnly = false
	sb.PlaceParams.ClientID = fmt.Sprintf("%d:%s", sb.ID, BET)

	sb.BeginPlace = time.Now().UnixNano()
	p.surebetMap.Store(sb.PlaceParams.ClientID, sb)
	order, err := p.PlaceOrder(p.ctx, sb.PlaceParams)
	if err != nil {
		p.log.Warn("place_error", zap.Error(err), zap.Any("sb", sb), zap.Duration("elapsed", time.Duration(time.Now().UnixNano()-sb.StartTime)))
		return
	}
	sb.Done = time.Now().UnixNano()
	sb.OrderID = order.ID
	p.saveSbCh <- sb
	//p.saveOrderCh <- order

	placeCounter.Inc()
	p.log.Info("success",
		zap.Any("sb", sb),
		zap.Int64("place_count", placeCounter.Load()),
		zap.Int64("fills_count", fillsCounter.Load()),
		zap.Duration("place_elapsed", time.Duration(sb.Done-sb.BeginPlace)),
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
	p.checkBalanceCh <- true
}

//if sb.BinTicker.BidPrice.IsZero() || sb.FtxTicker.BidPrice.IsZero() {
//p.log.Debug("price_zero", zap.Any("ftx", sb.FtxTicker), zap.Any("binance", sb.BinTicker))
//return
//}
