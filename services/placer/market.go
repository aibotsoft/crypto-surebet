package placer

import (
	"github.com/aibotsoft/crypto-surebet/pkg/store"
	"github.com/jinzhu/copier"
)

func (p *Placer) GetMarkets() error {
	//start := time.Now()
	resp, err := p.client.NewGetMarketsService().Do(p.ctx)
	if err != nil {
		return err
	}
	//fmt.Println(resp)
	var data []store.Market
	err = copier.Copy(&data, resp)
	if err != nil {
		return err
	}
	p.saveMarkets(data)
	err = p.store.SaveMarkets(&data)
	if err != nil {
		return err
	}
	//p.log.Debug("GetMarkets_done",
	//	zap.Int("count", len(data)),
	//	zap.Duration("elapsed", time.Since(start)),
	//	zap.Int("goroutine", runtime.NumGoroutine()))
	return nil
}

func (p *Placer) saveMarkets(data []store.Market) {
	p.marketLock.Lock()
	defer p.marketLock.Unlock()
	for _, m := range data {
		var me store.MarketEmb
		_ = copier.Copy(&me, m)
		p.marketMap[m.Name] = &me
	}
}

func (p *Placer) FindMarket(symbol string) *store.MarketEmb {
	p.marketLock.Lock()
	defer p.marketLock.Unlock()
	return p.marketMap[symbol]
}

func (p *Placer) Unlock(symbol string) {
	//p.symbolLock.Lock()
	//defer p.symbolLock.Unlock()
	//p.symbolMap[symbol].Unlock()
	//delete(p.symbolMap, symbol)
}
func (p *Placer) Lock(symbol string) chan int64 {
	//p.symbolLock.Lock()
	//defer p.symbolLock.Unlock()
	l, ok := p.symbolMap[symbol]
	if ok {
		return l
	}
	//p.log.Info("lock", zap.String("s", symbol))
	p.symbolMap[symbol] = make(chan int64, 1)
	return p.symbolMap[symbol]

	//got, ok := p.symbolMap[symbol]
	//if !ok {
	//	//fmt.Println("new_symbol", symbol)
	//	p.symbolMap[symbol] = id
	//	return id, true
	//}
}
