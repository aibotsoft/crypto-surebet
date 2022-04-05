package store

import (
	"context"
	"fmt"
	"github.com/aibotsoft/crypto-surebet/pkg/config"
	ftxapi "github.com/aibotsoft/ftx-api"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"
)

type Store struct {
	cfg *config.Config
	log *zap.Logger
	ctx context.Context
	db  *gorm.DB
}

func NewStore(cfg *config.Config, z *zap.Logger, ctx context.Context) (*Store, error) {
	logLevel := logger.Warn
	switch cfg.Postgres.LogLevel {
	case "info":
		logLevel = logger.Info
	case "warn":
		logLevel = logger.Warn
	case "error":
		logLevel = logger.Error
	}
	newLogger := logger.New(
		log.New(os.Stderr, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             1 * time.Second, // Slow SQL threshold
			LogLevel:                  logLevel,        // Log level
			IgnoreRecordNotFoundError: true,            // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,           // Disable color
		},
	)
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("connect_to_database_error: %w", err)
	}
	return &Store{
		log: z,
		cfg: cfg,
		ctx: ctx,
		db:  db,
	}, nil
}
func (s *Store) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}
func (s *Store) Migrate() error {
	ctx, cancel := context.WithTimeout(s.ctx, s.cfg.Postgres.Timeout)
	defer cancel()
	err := s.db.WithContext(ctx).AutoMigrate(
		&Account{},
		&Balance{},
		&Order{},
		&Market{},
		&Fills{},
		&Surebet{},
		&Heal{},
	)
	if err != nil {
		return fmt.Errorf("auto_migrate_error: %w", err)
	}
	return nil
}

func (s *Store) SaveAccount(resp *Account) error {
	ctx, cancel := context.WithTimeout(s.ctx, s.cfg.Postgres.Timeout)
	defer cancel()
	return s.db.WithContext(ctx).Save(resp).Error
}

func (s *Store) SaveBalances(balanceList *[]Balance) error {
	ctx, cancel := context.WithTimeout(s.ctx, s.cfg.Postgres.Timeout)
	defer cancel()
	return s.db.WithContext(ctx).Save(balanceList).Error
}

func (s *Store) SaveMarkets(data *[]Market) error {
	ctx, cancel := context.WithTimeout(s.ctx, s.cfg.Postgres.Timeout)
	defer cancel()
	return s.db.WithContext(ctx).Save(data).Error
}

func (s *Store) SaveOrders(apiOrderList []ftxapi.Order) error {
	ctx, cancel := context.WithTimeout(s.ctx, s.cfg.Postgres.Timeout)
	defer cancel()
	var data []Order
	err := copier.Copy(&data, apiOrderList)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status",
			"avg_fill_price",
			"filled_size",
			"remaining_size",
		}),
		//UpdateAll: true,
	}).Create(data).Error
}

func (s *Store) SaveOrder(order *Order) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&order).Error
	if err != nil {
		s.log.Error("save_order_error", zap.Error(err))
	}
}

func (s *Store) SaveSurebet(sb *Surebet) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := s.db.WithContext(ctx).Create(sb).Error
	if err != nil {
		s.log.Error("save_sb_error", zap.Error(err), zap.Any("sb", sb))
	}
}

func (s *Store) SaveFills(data *Fills) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.db.WithContext(ctx).Create(data).Error
	if err != nil {
		s.log.Error("save_fills_error", zap.Error(err))
	}
}

func (s *Store) SaveHeal(data *Heal) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(data).Error
	if err != nil {
		s.log.Error("save_heal_error", zap.Error(err))
	}
}

func (s *Store) DeleteSurebetByOrderID(orderID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.db.WithContext(ctx).Where("order_id=?", orderID).Delete(&Surebet{}).Error
	if err != nil {
		s.log.Error("delete_surebet_error", zap.Error(err), zap.Int64("order_id", orderID))
	}
}

func (s *Store) DeleteOrderByID(orderID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.db.WithContext(ctx).Delete(&Order{}, orderID).Error
	if err != nil {
		s.log.Error("delete_order_error", zap.Error(err), zap.Int64("order_id", orderID))
	}
}

func (s *Store) SelectHealByID(id int64) (*Heal, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var heal Heal
	err := s.db.Debug().WithContext(ctx).First(&heal, id).Error
	return &heal, err
}

func (s *Store) FindHealOrders(heal *Heal) {
	var orders []*Order
	err := s.db.Model(&heal).Association("Orders").Find(&orders)
	if err != nil {
		return
	}
	heal.Orders = orders
}

//func (s *Store) GetWallet(symbol string) (base *Wallet, quote *Wallet) {
//	baseStr := strings.Replace(symbol, "USDT", "", 1)
//	gotBase, _ := s.wallet.LoadOrStore(baseStr, &Wallet{Coin: baseStr})
//	base = gotBase.(*Wallet)
//	gotQuote, _ := s.wallet.Load("USDT")
//	quote = gotQuote.(*Wallet)
//	return
//}
//func (s *Store) SaveWallet(base *Wallet, quote *Wallet) {
//	s.wallet.Store(base.Coin, base)
//	s.wallet.Store("USDT", quote)
//	s.db.Save([]*Wallet{base, quote})
//}
//func (w *Wallet) BuyAvg() float64 {
//	var sum, avg float64
//	var count int
//	for i := 0; i < len(w.Orders); i++ {
//		if w.Orders[i].Side == BUY {
//			sum += w.Orders[i].Price
//			count++
//		}
//	}
//	if count > 0 {
//		avg = sum / float64(count)
//	}
//	return avg
//}
//func (w *Wallet) SellAvg() float64 {
//	var sum, avg float64
//	var count int
//	for i := 0; i < len(w.Orders); i++ {
//		if w.Orders[i].Side == SELL {
//			sum += w.Orders[i].Price
//			count++
//		}
//	}
//	if count > 0 {
//		avg = sum / float64(count)
//	}
//	return avg
//}

//func (w *Wallet) Profit() float64 {
//	if w.BuyCount > 0 && w.SellCount > 0 {
//		return w.SellAvg() - w.BuyAvg()
//	}
//	return 0
//}
//func (w *Wallet) ProfitAvg() float64 {
//	var sum, avg float64
//	for i := 0; i < len(w.Orders); i++ {
//		sum += w.Orders[i].Profit
//	}
//	if len(w.Orders) > 0 {
//		avg = sum / float64(len(w.Orders))
//	}
//	return avg
//}
//func (s *Store) quote() *Wallet {
//	got, _ := s.wallet.Load("USDT")
//	return got.(*Wallet)
//}
//func (s *Store) PortfolioSum() float64 {
//	var sum float64
//	for _, coin := range s.cfg.Markets {
//		//c.log.Info("sum", zap.String("symbol", symbol), zap.Float64("a", amount))
//		got, ok := s.wallet.Load(strings.ToUpper(coin))
//		if !ok {
//			continue
//		}
//		base := got.(*Wallet)
//		sum = sum + base.Amount*base.BidPrice
//	}
//	sum = sum + s.quote().Amount
//	return sum
//}
//func (s *Store) PrintStat() {
//	quote := s.quote()
//	var sum float64
//	//var profitAvg float64
//	var sellCount, buyCount int64
//	w := tabwriter.NewWriter(os.Stdout, 10, 1, 1, ' ', tabwriter.Debug)
//	fmt.Fprintf(w, "num\tcoin\tamount\tsells\tbuys\ttotal\tprofit_avg\tsell_avg\tbuy_avg\tsell_buy_diff\t\n")
//	for num, coin := range s.cfg.Markets {
//		got, ok := s.wallet.Load(strings.ToUpper(coin))
//		if !ok {
//			continue
//		}
//		b := got.(*Wallet)
//		sum = sum + b.Amount*b.BidPrice
//		//profitAvg += b.ProfitAvg()
//		sellCount += b.SellCount
//		buyCount += b.BuyCount
//		fmt.Fprintf(w, "%d\t%s\t%.4f\t%d\t%d\t%d\t%.4f\t%.4f\t%.4f\t%.4f\t\n",
//			num,
//			b.Coin,
//			b.Amount,
//			b.SellCount,
//			b.BuyCount,
//			b.SellCount+b.BuyCount,
//			b.ProfitAvg(),
//			b.SellAvg(),
//			b.BuyAvg(),
//			b.Profit(),
//		)
//	}
//	fmt.Fprintf(w, "%d\t%s\t%.4f\t%d\t%d\t%d\t%.4f\t%.4f\t%.4f\t%.4f\t\n",
//		0,
//		quote.Coin,
//		quote.Amount,
//		quote.SellCount,
//		quote.BuyCount,
//		quote.SellCount+quote.BuyCount,
//		float64(0),
//		float64(0),
//		float64(0),
//		float64(0),
//	)
//	//fmt.Fprintf(w, "%s\n", strings.Repeat("-", 70))
//	fmt.Fprintf(w, "%d\t%s\t%.4f\t%d\t%d\t%d\t%.4f\t%.4f\t%.4f\t\n",
//		999,
//		"TOTAL",
//		sum+quote.Amount,
//		sellCount,
//		buyCount,
//		sellCount+buyCount,
//		float64(0),
//		float64(0),
//		float64(0),
//	)
//	w.Flush()
//}
