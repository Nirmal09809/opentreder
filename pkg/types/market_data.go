package types

import (
	"time"
)

type Quote struct {
	Symbol        string          `json:"symbol"`
	BidPrice      decimal.Decimal `json:"bid_price"`
	BidSize       decimal.Decimal `json:"bid_size"`
	AskPrice      decimal.Decimal `json:"ask_price"`
	AskSize       decimal.Decimal `json:"ask_size"`
	Timestamp     time.Time       `json:"timestamp"`
	Exchange      Exchange        `json:"exchange"`
}

type Bar struct {
	Symbol    string          `json:"symbol"`
	Open      decimal.Decimal `json:"open"`
	High     decimal.Decimal `json:"high"`
	Low      decimal.Decimal `json:"low"`
	Close    decimal.Decimal `json:"close"`
	Volume   decimal.Decimal `json:"volume"`
	StartTime time.Time      `json:"start_time"`
	EndTime  time.Time      `json:"end_time"`
	Exchange Exchange        `json:"exchange"`
}

type Asset struct {
	Symbol           string    `json:"symbol"`
	Exchange        Exchange  `json:"exchange"`
	AssetType       AssetType `json:"asset_type"`
	Name            string    `json:"name"`
	Status          string    `json:"status"`
	Tradable        bool      `json:"tradable"`
	Marginable      bool      `json:"marginable"`
	Shortable       bool      `json:"shortable"`
	EasyToBorrow    bool      `json:"easy_to_borrow"`
	MaintenanceMode string    `json:"maintenance_mode"`
}

type MarketClock struct {
	Timestamp    time.Time `json:"timestamp"`
	IsOpen       bool      `json:"is_open"`
	NextOpen     time.Time `json:"next_open"`
	NextClose    time.Time `json:"next_close"`
	AfterHours   bool      `json:"after_hours"`
}

type CalendarDay struct {
	Date      string `json:"date"`
	Exchange  string `json:"exchange"`
	Open      string `json:"open"`
	Close     string `json:"close"`
	IsTrading bool   `json:"is_trading"`
	IsHalfDay bool   `json:"is_half_day"`
}

type OptionContract struct {
	Symbol            string          `json:"symbol"`
	ContractID       string          `json:"contract_id"`
	UnderlyingSymbol string          `json:"underlying_symbol"`
	ExpirationDate   time.Time       `json:"expiration_date"`
	StrikePrice      decimal.Decimal `json:"strike_price"`
	OptionType       string          `json:"option_type"`
	Style            string          `json:"style"`
	ExpirationType   string          `json:"expiration_type"`
	SharesPerContract int            `json:"shares_per_contract"`
	Multiplier       int             `json:"multiplier"`
}

type OptionQuote struct {
	Symbol        string          `json:"symbol"`
	BidPrice      decimal.Decimal `json:"bid_price"`
	BidSize       int             `json:"bid_size"`
	AskPrice      decimal.Decimal `json:"ask_price"`
	AskSize       int             `json:"ask_size"`
	UnderlyingPrice decimal.Decimal `json:"underlying_price"`
	ImpliedVolatility decimal.Decimal `json:"implied_volatility"`
	Delta         decimal.Decimal `json:"delta"`
	Gamma         decimal.Decimal `json:"gamma"`
	Theta         decimal.Decimal `json:"theta"`
	Vega          decimal.Decimal `json:"vega"`
	rho           decimal.Decimal `json:"rho"`
	Timestamp     time.Time       `json:"timestamp"`
}

type Transaction struct {
	ID              string          `json:"id"`
	AccountID       string          `json:"account_id"`
	Type            string          `json:"type"`
	Subtype         string          `json:"subtype"`
	Symbol          string          `json:"symbol"`
	Quantity        decimal.Decimal `json:"quantity"`
	Price           decimal.Decimal `json:"price"`
	Amount          decimal.Decimal `json:"amount"`
	Fee             decimal.Decimal `json:"fee"`
	Date            time.Time       `json:"date"`
}

type OrderRow struct {
	Symbol   string `json:"symbol"`
	Side     string `json:"side"`
	Type     string `json:"type"`
	Quantity string `json:"quantity"`
	Price    string `json:"price"`
}

type StrategyRow struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	PnL    string `json:"pnl"`
}
