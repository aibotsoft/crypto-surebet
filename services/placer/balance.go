package placer

import (
	"fmt"
	"github.com/aibotsoft/crypto-surebet/pkg/store"
	"github.com/jinzhu/copier"
	"github.com/shopspring/decimal"
)

func (p *Placer) GetBalances() error {
	//start := time.Now()
	resp, err := p.client.NewGetBalancesService().Do(p.ctx)
	if err != nil {
		return err
	}
	if len(resp) == 0 {
		return fmt.Errorf("balance_list_empty")
	}
	//p.log.Info("balance_resp", zap.Any("resp", resp))
	var data []store.Balance
	err = copier.Copy(&data, resp)
	if err != nil {
		return err
	}
	p.saveBalances(data)
	//total := p.BalanceTotal()
	err = p.store.SaveBalances(&data)
	if err != nil {
		return err
	}
	//p.log.Debug("GetBalances_done",
	//	zap.Int("count", len(data)),
	//	//zap.Int64("total", total.IntPart()),
	//	zap.Any("p.targetAmount", p.targetAmount),
	//	//zap.Duration("elapsed", time.Since(start)),
	//	zap.Bool("reused", Reused),
	//	zap.Bool("was_idle", WasIdle),
	//	zap.Duration("idle_time", IdleTime),
	//	zap.Int("goroutine", runtime.NumGoroutine()),
	//)
	return nil
}

func (p *Placer) BalanceAdd(coin string, usdValueDiff decimal.Decimal, coinValueDiff decimal.Decimal) {
	p.balanceLock.Lock()
	defer p.balanceLock.Unlock()
	got, ok := p.balanceMap[coin]
	if !ok {
		return
	}
	got.UsdValue = got.UsdValue.Add(usdValueDiff)
	got.Free = got.Free.Add(coinValueDiff)
}
func (p *Placer) BalanceSub(coin string, usdValueDiff decimal.Decimal, coinValueDiff decimal.Decimal) {
	p.balanceLock.Lock()
	defer p.balanceLock.Unlock()
	got, ok := p.balanceMap[coin]
	if !ok {
		return
	}
	got.UsdValue = got.UsdValue.Sub(usdValueDiff)
	got.Free = got.Free.Sub(coinValueDiff)
}
func (p *Placer) FindBalance(coin string) *store.BalanceEmb {
	p.balanceLock.Lock()
	defer p.balanceLock.Unlock()
	got, ok := p.balanceMap[coin]
	if !ok {
		return &store.BalanceEmb{}
	}
	return got
}

func (p *Placer) saveBalances(data []store.Balance) {
	p.balanceLock.Lock()
	defer p.balanceLock.Unlock()
	for _, b := range data {
		got, ok := p.balanceMap[b.Coin]
		if !ok {
			p.balanceMap[b.Coin] = &store.BalanceEmb{
				Free:     b.Free,
				Total:    b.Total,
				UsdValue: b.UsdValue,
			}
			continue
		}
		got.UsdValue = b.UsdValue
		got.Total = b.Total
		got.Free = b.Free
		//p.balanceMap[b.Coin] = &store.BalanceEmb{
		//	Free:     b.Free,
		//	Total:    b.Total,
		//	UsdValue: b.UsdValue,
		//}
	}
}
