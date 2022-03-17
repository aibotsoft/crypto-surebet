package store

import (
	"github.com/shopspring/decimal"
	"time"
)

//type ToEmbed struct {
//	Fuck   int64
//	Second string
//}
//type Demo struct {
//	ID    int64   `gorm:"primaryKey;autoIncrement:false"`
//	Embed ToEmbed `gorm:"embedded"`
//	Other ToEmbed `gorm:"embedded"`
//}
//type Order struct {
//	ID        int64 `gorm:"primaryKey"`
//	CreatedAt time.Time
//	UpdatedAt time.Time
//	DeletedAt gorm.DeletedAt `gorm:"index"`
//
//	Market string
//	Side   string
//	Price  float64
//	Type   string
//	Size   float64
//
//	Profit   float64
//	TimeDiff time.Duration
//	WalletID int64
//}
//type Wallet struct {
//	ID        int64 `gorm:"primaryKey"`
//	CreatedAt time.Time
//	UpdatedAt time.Time
//	DeletedAt gorm.DeletedAt `gorm:"index"`
//	Coin      string         `gorm:"uniqueIndex"`
//
//	LastBet   int64
//	SellCount int64
//	BuyCount  int64
//	BidPrice  float64
//	Amount    float64
//	Orders    []Order `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
//}

type Account struct {
	UpdatedAt                    time.Time       `json:"updated_at" gorm:"not null"`
	BackstopProvider             bool            `json:"backstopProvider"`
	Collateral                   float64         `json:"collateral"`
	FreeCollateral               float64         `json:"freeCollateral"`
	InitialMarginRequirement     float64         `json:"initialMarginRequirement"`
	Leverage                     float64         `json:"leverage"`
	Liquidating                  bool            `json:"liquidating"`
	MaintenanceMarginRequirement float64         `json:"maintenanceMarginRequirement"`
	MakerFee                     decimal.Decimal `json:"makerFee" gorm:"type:numeric not null"`
	MarginFraction               float64         `json:"marginFraction"`
	OpenMarginFraction           float64         `json:"openMarginFraction"`
	TakerFee                     decimal.Decimal `json:"takerFee" gorm:"type:numeric not null"`
	TotalAccountValue            float64         `json:"totalAccountValue"`
	TotalPositionSize            float64         `json:"totalPositionSize"`
	Username                     string          `json:"username" gorm:"primaryKey"`
	//Positions                    []Position     `json:"positions"`
}
type Balance struct {
	Coin                   string          `json:"coin" gorm:"primaryKey"`
	Free                   decimal.Decimal `json:"free" gorm:"type:numeric not null"`
	Total                  decimal.Decimal `json:"total" gorm:"type:numeric not null"`
	UsdValue               decimal.Decimal `json:"usd_value" gorm:"type:numeric not null"`
	AvailableWithoutBorrow decimal.Decimal `json:"available" gorm:"type:numeric not null"`
	UpdatedAt              time.Time       `json:"updated_at" gorm:"not null"`
}

type OrderType string

const (
	OrderTypeLimit  OrderType = "limit"
	OrderTypeMarket OrderType = "market"
)

type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

type OrderStatus string

const (
	OrderStatusNew       OrderStatus = "new"
	OrderStatusOpen      OrderStatus = "open"
	OrderStatusFilled    OrderStatus = "filled"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusClosed    OrderStatus = "closed"
	OrderStatusTriggered OrderStatus = "triggered"
)

//type OrderInfo struct {
//	SbID int64 `json:"sb_id" gorm:"index"`
//}

type Order struct {
	CreatedAt     time.Time   `json:"createdAt" gorm:"not null"`
	UpdatedAt     time.Time   `json:"updated_at" gorm:"not null"`
	Market        string      `json:"market" gorm:"not null"`
	Side          Side        `json:"side" gorm:"not null"`
	Status        OrderStatus `json:"status" gorm:"not null"`
	Type          OrderType   `json:"type" gorm:"not null"`
	ClientID      *string     `json:"clientId"`
	Price         float64     `json:"price" gorm:"not null"`
	AvgFillPrice  float64     `json:"avgFillPrice" `
	Size          float64     `json:"size"`
	FilledSize    float64     `json:"filledSize" gorm:"not null"`
	RemainingSize float64     `json:"remainingSize"`
	ID            int64       `json:"id" gorm:"primaryKey;autoIncrement:false"`
	Ioc           bool        `json:"ioc"`
	PostOnly      bool        `json:"postOnly"`
	ReduceOnly    bool        `json:"reduceOnly"`
	ClosedAt      *int64      `json:"closed_at"`

	//OrderInfo     *OrderInfo  `json:"order_info,omitempty" gorm:"embedded"`
}
type Market struct {
	UpdatedAt             time.Time `json:"updated_at" gorm:"not null"`
	Name                  string    `json:"name" gorm:"primaryKey"`
	BaseCurrency          *string   `json:"baseCurrency"`
	QuoteCurrency         *string   `json:"quoteCurrency"`
	QuoteVolume24H        float64   `json:"quoteVolume24h"`
	Change1H              float64   `json:"change1h"`
	Change24H             float64   `json:"change24h"`
	ChangeBod             float64   `json:"changeBod"`
	HighLeverageFeeExempt bool      `json:"highLeverageFeeExempt"`
	MinProvideSize        float64   `json:"minProvideSize"`
	Type                  string    `json:"type"`
	Underlying            string    `json:"underlying"`
	Enabled               bool      `json:"enabled"`
	Ask                   float64   `json:"ask"`
	Bid                   float64   `json:"bid"`
	Last                  float64   `json:"last"`
	PostOnly              bool      `json:"postOnly"`
	Price                 float64   `json:"price"`
	PriceIncrement        float64   `json:"priceIncrement"`
	SizeIncrement         float64   `json:"sizeIncrement"`
	Restricted            bool      `json:"restricted"`
	VolumeUsd24H          float64   `json:"volumeUsd24h"`
}

type TickerData struct {
	Symbol          string          `json:"s" gorm:"not null"`
	BidPrice        decimal.Decimal `json:"b" gorm:"type:numeric not null"`
	BidQty          decimal.Decimal `json:"B" gorm:"type:numeric not null"`
	AskPrice        decimal.Decimal `json:"a" gorm:"type:numeric not null"`
	AskQty          decimal.Decimal `json:"A" gorm:"type:numeric not null"`
	ServerTime      int64           `json:"st" gorm:"not null"`
	ReceiveTime     int64           `json:"rt" gorm:"not null"`
	PrevBidPrice    decimal.Decimal `json:"pb" gorm:"type:numeric"`
	PrevBidQty      decimal.Decimal `json:"pB" gorm:"type:numeric"`
	PrevAskPrice    decimal.Decimal `json:"pa" gorm:"type:numeric"`
	PrevAskQty      decimal.Decimal `json:"pA" gorm:"type:numeric"`
	PrevServerTime  int64           `json:"pst"`
	PrevReceiveTime int64           `json:"prt"`
}
type BalanceEmb struct {
	//Coin      string          `json:"coin" gorm:"primaryKey"`
	Free     decimal.Decimal `json:"free" gorm:"type:numeric not null"`
	Total    decimal.Decimal `json:"total" gorm:"type:numeric not null"`
	UsdValue decimal.Decimal `json:"usdValue" gorm:"type:numeric not null"`
	//UpdatedAt *time.Time      `json:"updated_at"`
}
type MarketEmb struct {
	//Name           string  `json:"name"`
	BaseCurrency   string          `json:"baseCurrency" gorm:"not null"`
	QuoteCurrency  string          `json:"quoteCurrency" gorm:"not null"`
	MinProvideSize decimal.Decimal `json:"minProvideSize" gorm:"type:numeric not null"`
	SizeIncrement  decimal.Decimal `json:"sizeIncrement" gorm:"type:numeric not null"`
	PriceIncrement decimal.Decimal `json:"priceIncrement" gorm:"type:numeric not null"`
	Change1H       decimal.Decimal `json:"-" gorm:"type:numeric not null"`
	Change24H      decimal.Decimal `json:"-" gorm:"type:numeric not null"`
	ChangeBod      decimal.Decimal `json:"-" gorm:"type:numeric not null"`
	QuoteVolume24H int64           `json:"-" gorm:"not null"`
	VolumeUsd24H   int64           `json:"-" gorm:"not null"`
	//Type           string  `json:"type"`
	//Underlying     string  `json:"underlying"`
	//Enabled        bool    `json:"enabled"`
	//Ask            float64 `json:"ask"`
	//Bid            float64 `json:"bid"`
	//Last           float64 `json:"last"`
	//PostOnly       bool    `json:"postOnly"`
	//Price          float64 `json:"price"`
	//Restricted     bool    `json:"restricted"`
}
type Fills struct {
	BaseCurrency  *string   `json:"baseCurrency" gorm:"not null"`
	Fee           float64   `json:"fee"  gorm:"not null"`
	FeeCurrency   string    `json:"feeCurrency"  gorm:"not null"`
	FeeRate       float64   `json:"feeRate" gorm:"not null"`
	ID            int64     `json:"id" gorm:"primaryKey;autoIncrement:false;not null"`
	Liquidity     string    `json:"liquidity" gorm:"not null"`
	Market        string    `json:"market" gorm:"not null"`
	OrderID       int64     `json:"orderId" gorm:"index;not null"`
	Price         float64   `json:"price" gorm:"not null"`
	QuoteCurrency string    `json:"quoteCurrency" gorm:"not null"`
	Side          Side      `json:"side" gorm:"not null"`
	Size          float64   `json:"size" gorm:"not null"`
	Time          time.Time `json:"time" gorm:"not null"`
	TradeID       int64     `json:"tradeId" gorm:"not null"`
	Type          string    `json:"type" gorm:"not null"`
}
type PlaceParamsEmb struct {
	Market   string          `json:"market" gorm:"-"`
	Side     Side            `json:"side" gorm:"not null"`
	Price    decimal.Decimal `json:"price" gorm:"type:numeric not null"`
	Type     OrderType       `json:"type" gorm:"not null"`
	Size     decimal.Decimal `json:"size" gorm:"type:numeric not null"`
	Ioc      bool            `json:"ioc"`
	PostOnly bool            `json:"postOnly"`
	ClientID string          `json:"clientId" gorm:"-"`
}
type Surebet struct {
	CreatedAt   time.Time `json:"-"`
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement:false"`
	LastBinTime int64     `json:"last_bin_time"`
	StartTime   int64     `json:"start"`
	BeginPlace  int64     `json:"begin_place"`
	Done        int64     `json:"done"`
	//FtxDelay         int16           `json:"ftx_delay,omitempty" gorm:"type:smallint not null"`
	//BinFtxTimeDiffMs int32           `json:"bin_ftx_time_diff_ms" gorm:"type:integer not null"`
	//BuyDiff        decimal.Decimal `json:"buy_diff" gorm:"type:numeric not null"`
	//SellDiff       decimal.Decimal `json:"sell_diff" gorm:"type:numeric not null"`
	FtxSpread  decimal.Decimal `json:"ftx_spread" gorm:"type:numeric not null"`
	BinSpread  decimal.Decimal `json:"bin_spread" gorm:"type:numeric not null"`
	BuyProfit  decimal.Decimal `json:"buy_profit" gorm:"type:numeric not null"`
	SellProfit decimal.Decimal `json:"sell_profit" gorm:"type:numeric not null"`
	//MinProfit       decimal.Decimal `json:"min_profit" gorm:"type:numeric not null"`
	TargetProfit    decimal.Decimal `json:"target_profit" gorm:"type:numeric not null"`
	Profit          decimal.Decimal `json:"profit" gorm:"type:numeric not null"`
	RequiredProfit  decimal.Decimal `json:"required_profit" gorm:"type:numeric not null"`
	AmountCoef      decimal.Decimal `json:"amount_coef" gorm:"type:numeric not null"`
	ProfitInc       decimal.Decimal `json:"profit_inc" gorm:"type:numeric not null"`
	Volume          decimal.Decimal `json:"volume" gorm:"type:numeric not null"`
	MaxStake        decimal.Decimal `json:"max_stake" gorm:"type:numeric not null"`
	PlaceParams     PlaceParamsEmb  `json:"place" gorm:"embedded;embeddedPrefix:place_"`
	BinTicker       *TickerData     `json:"b" gorm:"embedded;embeddedPrefix:bin_"`
	FtxTicker       *TickerData     `json:"o" gorm:"embedded;embeddedPrefix:ftx_"`
	BaseBalance     *BalanceEmb     `json:"base_balance,omitempty" gorm:"embedded;embeddedPrefix:base_"`
	QuoteBalance    *BalanceEmb     `json:"quote_balance,omitempty" gorm:"embedded;embeddedPrefix:quote_"`
	MakerFee        decimal.Decimal `json:"maker_fee" gorm:"type:numeric not null"`
	TakerFee        decimal.Decimal `json:"taker_fee" gorm:"type:numeric not null"`
	Market          *MarketEmb      `json:"market,omitempty" gorm:"embedded"`
	ProfitPriceDiff decimal.Decimal `json:"profit_price_diff" gorm:"type:numeric"`
	ConnReused      bool            `json:"conn_reused"`
	BinVolume       decimal.Decimal `json:"bin_volume" gorm:"type:numeric"`
	Price           decimal.Decimal `json:"price" gorm:"type:numeric"`
	ProfitSubSpread decimal.Decimal `json:"profit_sub_spread" gorm:"type:numeric"`
	BinPrice        decimal.Decimal `json:"bin_price" gorm:"type:numeric"`
	ProfitSubFee    decimal.Decimal `json:"profit_sub_fee" gorm:"type:numeric"`
	RealFee         decimal.Decimal `json:"real_fee" gorm:"type:numeric"`
	TargetAmount    decimal.Decimal `json:"target_amount" gorm:"type:numeric"`
	OrderID         int64           `json:"order_id"`
	AvgPriceDiff    decimal.Decimal `json:"avg_price_diff" gorm:"type:numeric"`
	MaxPriceDiff    decimal.Decimal `json:"max_price_diff" gorm:"type:numeric"`
	MinPriceDiff    decimal.Decimal `json:"min_price_diff" gorm:"type:numeric"`
	ProfitSubAvg    decimal.Decimal `json:"profit_sub_avg" gorm:"type:numeric"`
}
type Heal struct {
	CreatedAt    time.Time       `json:"-" gorm:"not null"`
	ID           int64           `json:"id" gorm:"primaryKey;autoIncrement:false;not null"`
	Start        int64           `json:"start" gorm:"not null"`
	Done         int64           `json:"done" gorm:"not null"`
	OrderID      int64           `json:"order_id" gorm:"index"`
	PlaceParams  PlaceParamsEmb  `json:"place" gorm:"embedded;embeddedPrefix:place_"`
	FilledSize   decimal.Decimal `json:"filled_size" gorm:"type:numeric not null"`
	AvgFillPrice decimal.Decimal `json:"avg_fill_price" gorm:"type:numeric not null"`
	FeePart      decimal.Decimal `json:"fee_part" gorm:"type:numeric not null"`
	ProfitPart   decimal.Decimal `json:"profit_part" gorm:"type:numeric not null"`
	ErrorMsg     *string         `json:"error_msg"`
}
