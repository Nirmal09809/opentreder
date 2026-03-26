package types

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type AssetType string

const (
	AssetTypeCrypto   AssetType = "crypto"
	AssetTypeStock    AssetType = "stock"
	AssetTypeForex    AssetType = "forex"
	AssetTypeOption   AssetType = "option"
	AssetTypeFutures  AssetType = "futures"
	AssetTypeIndex    AssetType = "index"
	AssetTypeCommodity AssetType = "commodity"
)

type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
)

type OrderType string

const (
	OrderTypeMarket     OrderType = "market"
	OrderTypeLimit      OrderType = "limit"
	OrderTypeStop       OrderType = "stop"
	OrderTypeStopLimit  OrderType = "stop_limit"
	OrderTypeTrailing   OrderType = "trailing_stop"
	OrderTypeIOC        OrderType = "ioc"
	OrderTypeFOK        OrderType = "fok"
	OrderTypeIceberg    OrderType = "iceberg"
	OrderTypeTWAP       OrderType = "twap"
	OrderTypeVWAP       OrderType = "vwap"
)

type OrderStatus string

const (
	OrderStatusPending      OrderStatus = "pending"
	OrderStatusOpen         OrderStatus = "open"
	OrderStatusPartiallyFilled OrderStatus = "partial"
	OrderStatusFilled       OrderStatus = "filled"
	OrderStatusCancelled    OrderStatus = "cancelled"
	OrderStatusRejected     OrderStatus = "rejected"
	OrderStatusExpired      OrderStatus = "expired"
)

type PositionSide string

const (
	PositionSideLong  PositionSide = "long"
	PositionSideShort PositionSide = "short"
	PositionSideBoth  PositionSide = "both"
)

type Exchange string

const (
	ExchangeBinance      Exchange = "binance"
	ExchangeBybit        Exchange = "bybit"
	ExchangeCoinbase     Exchange = "coinbase"
	ExchangeKraken       Exchange = "kraken"
	ExchangeOKX          Exchange = "okx"
	ExchangeAlpaca       Exchange = "alpaca"
	ExchangeInteractiveBrokers Exchange = "interactive_brokers"
	ExchangeTradeStation Exchange = "tradestation"
	ExchangeOANDA        Exchange = "oanda"
	ExchangePolygon      Exchange = "polygon"
)

type MarketType string

const (
	MarketTypeSpot       MarketType = "spot"
	MarketTypeFutures    MarketType = "futures"
	MarketTypePerpetual  MarketType = "perpetual"
	MarketTypeMargin     MarketType = "margin"
	MarketTypeOption     MarketType = "option"
)

type TimeInForce string

const (
	TimeInForceGTC TimeInForce = "gtc"
	TimeInForceIOC TimeInForce = "ioc"
	TimeInForceFOK TimeInForce = "fok"
	TimeInForceGTX TimeInForce = "gtx"
	TimeInForceGTT TimeInForce = "gtt"
)

type Order struct {
	ID              uuid.UUID       `json:"id" db:"id"`
	ClientOrderID   string          `json:"client_order_id" db:"client_order_id"`
	Exchange        Exchange        `json:"exchange" db:"exchange"`
	Symbol          string          `json:"symbol" db:"symbol"`
	AssetType       AssetType       `json:"asset_type" db:"asset_type"`
	MarketType      MarketType      `json:"market_type" db:"market_type"`
	Side            OrderSide       `json:"side" db:"side"`
	Type            OrderType       `json:"type" db:"type"`
	Status          OrderStatus     `json:"status" db:"status"`
	Price           decimal.Decimal `json:"price" db:"price"`
	StopPrice       decimal.Decimal `json:"stop_price" db:"stop_price"`
	Quantity        decimal.Decimal `json:"quantity" db:"quantity"`
	FilledQuantity  decimal.Decimal `json:"filled_quantity" db:"filled_quantity"`
	RemainingQty    decimal.Decimal `json:"remaining_qty" db:"remaining_qty"`
	AvgFillPrice    decimal.Decimal `json:"avg_fill_price" db:"avg_fill_price"`
	Commission      decimal.Decimal `json:"commission" db:"commission"`
	CommissionAsset string          `json:"commission_asset" db:"commission_asset"`
	TimeInForce     TimeInForce     `json:"time_in_force" db:"time_in_force"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
	FilledAt        *time.Time      `json:"filled_at,omitempty" db:"filled_at"`
	CancelledAt     *time.Time      `json:"cancelled_at,omitempty" db:"cancelled_at"`
	StrategyID      uuid.UUID       `json:"strategy_id,omitempty" db:"strategy_id"`
	Tags            []string        `json:"tags,omitempty" db:"-"`
	Metadata        JSONMap         `json:"metadata,omitempty" db:"-"`
}

type OrderUpdate struct {
	OrderID         uuid.UUID       `json:"order_id"`
	Status          OrderStatus     `json:"status"`
	FilledQuantity  decimal.Decimal `json:"filled_quantity"`
	AvgFillPrice    decimal.Decimal `json:"avg_fill_price"`
	Commission      decimal.Decimal `json:"commission"`
	Message         string          `json:"message,omitempty"`
	Timestamp       time.Time       `json:"timestamp"`
}

type Position struct {
	ID            uuid.UUID        `json:"id" db:"id"`
	Exchange      Exchange         `json:"exchange" db:"exchange"`
	Symbol        string           `json:"symbol" db:"symbol"`
	AssetType     AssetType        `json:"asset_type" db:"asset_type"`
	Side          PositionSide     `json:"side" db:"side"`
	Quantity      decimal.Decimal  `json:"quantity" db:"quantity"`
	AvgEntryPrice decimal.Decimal  `json:"avg_entry_price" db:"avg_entry_price"`
	CurrentPrice  decimal.Decimal  `json:"current_price" db:"current_price"`
	UnrealizedPnL decimal.Decimal  `json:"unrealized_pnl" db:"unrealized_pnl"`
	RealizedPnL   decimal.Decimal  `json:"realized_pnl" db:"realized_pnl"`
	ROI           decimal.Decimal  `json:"roi" db:"roi"`
	Leverage      decimal.Decimal  `json:"leverage" db:"leverage"`
	IsolatedMargin decimal.Decimal `json:"isolated_margin" db:"isolated_margin"`
	LiquidationPrice decimal.Decimal `json:"liquidation_price" db:"liquidation_price"`
	OpenedAt      time.Time        `json:"opened_at" db:"opened_at"`
	UpdatedAt     time.Time        `json:"updated_at" db:"updated_at"`
	StrategyID   uuid.UUID        `json:"strategy_id,omitempty" db:"strategy_id"`
}

type Portfolio struct {
	ID            uuid.UUID              `json:"id" db:"id"`
	Name          string                 `json:"name" db:"name"`
	TotalValue    decimal.Decimal       `json:"total_value" db:"total_value"`
	CashBalance   decimal.Decimal       `json:"cash_balance" db:"cash_balance"`
	Equity        decimal.Decimal       `json:"equity" db:"equity"`
	BuyingPower   decimal.Decimal       `json:"buying_power" db:"buying_power"`
	MarginUsed    decimal.Decimal       `json:"margin_used" db:"margin_used"`
	MarginAvailable decimal.Decimal    `json:"margin_available" db:"margin_available"`
	UnrealizedPnL decimal.Decimal       `json:"unrealized_pnl" db:"unrealized_pnl"`
	RealizedPnL   decimal.Decimal       `json:"realized_pnl" db:"realized_pnl"`
	DayPnL        decimal.Decimal       `json:"day_pnl" db:"day_pnl"`
	Positions     map[string]*Position `json:"positions,omitempty" db:"-"`
	Allocations   map[string]decimal.Decimal `json:"allocations,omitempty" db:"-"`
	UpdatedAt     time.Time             `json:"updated_at" db:"updated_at"`
}

type Candle struct {
	Symbol    string          `json:"symbol" db:"symbol"`
	Exchange  Exchange        `json:"exchange" db:"exchange"`
	Timeframe string          `json:"timeframe" db:"timeframe"`
	Timestamp time.Time       `json:"timestamp" db:"timestamp"`
	Open      decimal.Decimal `json:"open" db:"open"`
	High      decimal.Decimal `json:"high" db:"high"`
	Low       decimal.Decimal `json:"low" db:"low"`
	Close     decimal.Decimal `json:"close" db:"close"`
	Volume    decimal.Decimal `json:"volume" db:"volume"`
	QuoteVolume decimal.Decimal `json:"quote_volume" db:"quote_volume"`
	TakerBuyVolume decimal.Decimal `json:"taker_buy_volume" db:"taker_buy_volume"`
	Trades    int64           `json:"trades" db:"trades"`
	Closed     bool           `json:"closed" db:"closed"`
}

type Ticker struct {
	Symbol            string          `json:"symbol" db:"symbol"`
	Exchange          Exchange        `json:"exchange" db:"exchange"`
	LastPrice         decimal.Decimal `json:"last_price" db:"last_price"`
	BidPrice          decimal.Decimal `json:"bid_price" db:"bid_price"`
	BidQty            decimal.Decimal `json:"bid_qty" db:"bid_qty"`
	AskPrice          decimal.Decimal `json:"ask_price" db:"ask_price"`
	AskQty            decimal.Decimal `json:"ask_qty" db:"ask_qty"`
	Volume24h         decimal.Decimal `json:"volume_24h" db:"volume_24h"`
	QuoteVolume24h    decimal.Decimal `json:"quote_volume_24h" db:"quote_volume_24h"`
	High24h           decimal.Decimal `json:"high_24h" db:"high_24h"`
	Low24h            decimal.Decimal `json:"low_24h" db:"low_24h"`
	PriceChange24h    decimal.Decimal `json:"price_change_24h" db:"price_change_24h"`
	PriceChangePct24h decimal.Decimal `json:"price_change_pct_24h" db:"price_change_pct_24h"`
	Timestamp         time.Time       `json:"timestamp" db:"timestamp"`
}

type OrderBook struct {
	Symbol    string            `json:"symbol" db:"symbol"`
	Exchange  Exchange          `json:"exchange" db:"exchange"`
	Bids      []OrderBookLevel  `json:"bids" db:"-"`
	Asks      []OrderBookLevel  `json:"asks" db:"-"`
	Timestamp time.Time         `json:"timestamp" db:"timestamp"`
}

type OrderBookLevel struct {
	Price    decimal.Decimal `json:"price"`
	Quantity decimal.Decimal `json:"quantity"`
}

type Trade struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	Exchange    Exchange        `json:"exchange" db:"exchange"`
	Symbol      string          `json:"symbol" db:"symbol"`
	Side        OrderSide       `json:"side" db:"side"`
	Price       decimal.Decimal `json:"price" db:"price"`
	Quantity    decimal.Decimal `json:"quantity" db:"quantity"`
	QuoteQty    decimal.Decimal `json:"quote_qty" db:"quote_qty"`
	Commission  decimal.Decimal `json:"commission" db:"commission"`
	Timestamp   time.Time       `json:"timestamp" db:"timestamp"`
	OrderID     uuid.UUID       `json:"order_id" db:"order_id"`
	IsBuyerMaker bool           `json:"is_buyer_maker" db:"is_buyer_maker"`
}

type Balance struct {
	Asset         string          `json:"asset" db:"asset"`
	Free          decimal.Decimal `json:"free" db:"free"`
	Locked        decimal.Decimal `json:"locked" db:"locked"`
	Total         decimal.Decimal `json:"total" db:"total"`
	USDValue      decimal.Decimal `json:"usd_value" db:"usd_value"`
	Exchange      Exchange        `json:"exchange" db:"exchange"`
}

type Market struct {
	Symbol           string           `json:"symbol" db:"symbol"`
	Exchange        Exchange         `json:"exchange" db:"exchange"`
	AssetType        AssetType        `json:"asset_type" db:"asset_type"`
	BaseAsset        string           `json:"base_asset" db:"base_asset"`
	QuoteAsset       string           `json:"quote_asset" db:"quote_asset"`
	Status           string           `json:"status" db:"status"`
	MinQty           decimal.Decimal  `json:"min_qty" db:"min_qty"`
	MaxQty           decimal.Decimal  `json:"max_qty" db:"max_qty"`
	StepSize         decimal.Decimal  `json:"step_size" db:"step_size"`
	MinNotional      decimal.Decimal  `json:"min_notional" db:"min_notional"`
	PricePrecision   int              `json:"price_precision" db:"price_precision"`
	QuantityPrecision int             `json:"quantity_precision" db:"quantity_precision"`
	MarginEnabled    bool             `json:"margin_enabled" db:"margin_enabled"`
	ContractType     string           `json:"contract_type,omitempty" db:"contract_type"`
	DeliveryDate     *time.Time       `json:"delivery_date,omitempty" db:"delivery_date"`
	ExpirationDate   *time.Time       `json:"expiration_date,omitempty" db:"expiration_date"`
}

type Strategy struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	Type        string          `json:"type" db:"type"`
	Description string          `json:"description" db:"description"`
	Enabled     bool            `json:"enabled" db:"enabled"`
	Mode        StrategyMode     `json:"mode" db:"mode"`
	Config      JSONMap         `json:"config" db:"-"`
	Parameters  JSONMap         `json:"parameters" db:"-"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

type StrategyMode string

const (
	StrategyModeLive     StrategyMode = "live"
	StrategyModePaper   StrategyMode = "paper"
	StrategyModeBacktest StrategyMode = "backtest"
	StrategyModeDryRun  StrategyMode = "dryrun"
)

type Signal struct {
	ID           uuid.UUID       `json:"id" db:"id"`
	StrategyID   uuid.UUID       `json:"strategy_id" db:"strategy_id"`
	Symbol       string          `json:"symbol" db:"symbol"`
	Exchange     Exchange        `json:"exchange" db:"exchange"`
	Action       SignalAction    `json:"action" db:"action"`
	Strength     decimal.Decimal `json:"strength" db:"strength"`
	Price        decimal.Decimal `json:"price" db:"price"`
	StopLoss     decimal.Decimal `json:"stop_loss" db:"stop_loss"`
	TakeProfit   decimal.Decimal `json:"take_profit" db:"take_profit"`
	Quantity     decimal.Decimal `json:"quantity" db:"quantity"`
	Reason       string          `json:"reason" db:"reason"`
	Confidence   decimal.Decimal `json:"confidence" db:"confidence"`
	Metadata     JSONMap         `json:"metadata,omitempty" db:"-"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
}

type SignalAction string

const (
	SignalActionBuy  SignalAction = "buy"
	SignalActionSell SignalAction = "sell"
	SignalActionHold SignalAction = "hold"
	SignalActionClose SignalAction = "close"
)

type RiskLimit struct {
	Type      string           `json:"type" db:"type"`
	MaxPositionSize decimal.Decimal `json:"max_position_size" db:"max_position_size"`
	MaxOrderSize decimal.Decimal `json:"max_order_size" db:"max_order_size"`
	MaxDailyLoss decimal.Decimal `json:"max_daily_loss" db:"max_daily_loss"`
	MaxDrawdown decimal.Decimal `json:"max_drawdown" db:"max_drawdown"`
}

type Account struct {
	ID              uuid.UUID          `json:"id" db:"id"`
	AccountID       string             `json:"account_id" db:"account_id"`
	Exchange        Exchange           `json:"exchange" db:"exchange"`
	AccountType     string             `json:"account_type" db:"account_type"`
	Currency        string             `json:"currency" db:"currency"`
	BuyingPower     decimal.Decimal    `json:"buying_power" db:"buying_power"`
	Cash            decimal.Decimal    `json:"cash" db:"cash"`
	PortfolioValue  decimal.Decimal    `json:"portfolio_value" db:"portfolio_value"`
	TradingEnabled  bool               `json:"trading_enabled" db:"trading_enabled"`
	Balances        map[string]*Balance `json:"balances,omitempty" db:"-"`
	Enabled         bool               `json:"enabled" db:"enabled"`
	CreatedAt       time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at" db:"updated_at"`
}

type PnLReport struct {
	TotalTrades        int              `json:"total_trades"`
	WinningTrades      int              `json:"winning_trades"`
	LosingTrades        int              `json:"losing_trades"`
	WinRate            decimal.Decimal  `json:"win_rate"`
	TotalPnL           decimal.Decimal  `json:"total_pnl"`
	AvgWin             decimal.Decimal  `json:"avg_win"`
	AvgLoss            decimal.Decimal  `json:"avg_loss"`
	ProfitFactor       decimal.Decimal  `json:"profit_factor"`
	MaxDrawdown        decimal.Decimal  `json:"max_drawdown"`
	SharpeRatio        decimal.Decimal  `json:"sharpe_ratio"`
	SortinoRatio       decimal.Decimal  `json:"sortino_ratio"`
	CalmarRatio        decimal.Decimal  `json:"calmar_ratio"`
	MaxConsecutiveWins int              `json:"max_consecutive_wins"`
	MaxConsecutiveLoss int              `json:"max_consecutive_losses"`
}

type JSONMap map[string]interface{}

type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

type Timeframe string

const (
	Timeframe1m  Timeframe = "1m"
	Timeframe5m  Timeframe = "5m"
	Timeframe15m Timeframe = "15m"
	Timeframe30m Timeframe = "30m"
	Timeframe1h  Timeframe = "1h"
	Timeframe4h  Timeframe = "4h"
	Timeframe1d  Timeframe = "1d"
	Timeframe1w  Timeframe = "1w"
	Timeframe1M  Timeframe = "1M"
)

var AllTimeframes = []Timeframe{
	Timeframe1m, Timeframe5m, Timeframe15m, Timeframe30m,
	Timeframe1h, Timeframe4h, Timeframe1d, Timeframe1w, Timeframe1M,
}

func (t Timeframe) Duration() time.Duration {
	switch t {
	case Timeframe1m:
		return time.Minute
	case Timeframe5m:
		return 5 * time.Minute
	case Timeframe15m:
		return 15 * time.Minute
	case Timeframe30m:
		return 30 * time.Minute
	case Timeframe1h:
		return time.Hour
	case Timeframe4h:
		return 4 * time.Hour
	case Timeframe1d:
		return 24 * time.Hour
	case Timeframe1w:
		return 7 * 24 * time.Hour
	case Timeframe1M:
		return 30 * 24 * time.Hour
	default:
		return time.Minute
	}
}
