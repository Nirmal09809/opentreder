package binance

// ---------------------------------------------------------------------------
// Exchange Info
// ---------------------------------------------------------------------------

type ExchangeInfo struct {
	Timezone   string   `json:"timezone"`
	ServerTime int64    `json:"serverTime"`
	Symbols    []Symbol `json:"symbols"`
}

type Symbol struct {
	Symbol          string      `json:"symbol"`
	Status         string      `json:"status"`
	BaseAsset      string      `json:"baseAsset"`
	BaseAssetType  string      `json:"baseAssetType"`
	QuoteAsset     string      `json:"quoteAsset"`
	QuoteAssetType string      `json:"quoteAssetType"`
	OrderTypes     []string    `json:"orderTypes"`
	IcebergAllowed bool        `json:"icebergAllowed"`
	FloorOrderQty  bool        `json:"floorOrderQty"`
	Filters        []Filter    `json:"filters"`
}

type Filter struct {
	FilterType string `json:"filterType"`
	MinPrice   string `json:"minPrice,omitempty"`
	MaxPrice   string `json:"maxPrice,omitempty"`
	TickSize   string `json:"tickSize,omitempty"`
	MinQty     string `json:"minQty,omitempty"`
	MaxQty     string `json:"maxQty,omitempty"`
	StepSize   string `json:"stepSize,omitempty"`
}

// ---------------------------------------------------------------------------
// Account
// ---------------------------------------------------------------------------

type AccountResponse struct {
	MakerCommission  int64          `json:"makerCommission"`
	TakerCommission  int64          `json:"takerCommission"`
	BuyerCommission int64          `json:"buyerCommission"`
	SellerCommission int64          `json:"sellerCommission"`
	CanTrade        bool           `json:"canTrade"`
	CanWithdraw     bool           `json:"canWithdraw"`
	CanDeposit      bool           `json:"canDeposit"`
	UpdateTime      int64          `json:"updateTime"`
	AccountType     string         `json:"accountType"`
	Permissions     []string       `json:"permissions"`
	Balances       []Balance      `json:"balances"`
}

type Balance struct {
	Asset   string `json:"asset"`
	Free    string `json:"free"`
	Locked  string `json:"locked"`
}

// ---------------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------------

type OrderResponse struct {
	OrderID       int64  `json:"orderId"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	Price         string `json:"price"`
	StopPrice     string `json:"stopPrice"`
	OrigQty       string `json:"origQty"`
	ExecutedQty   string `json:"executedQty"`
	Status        string `json:"status"`
	TimeInForce   string `json:"timeInForce"`
	QuoteOrderQty string `json:"quoteOrderQty"`
	Time          int64  `json:"time"`
	UpdateTime    int64  `json:"updateTime"`
	IsWorking     bool   `json:"isWorking"`
}

type OrderRequest struct {
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	Quantity      string `json:"quantity"`
	Price         string `json:"price,omitempty"`
	StopPrice     string `json:"stopPrice,omitempty"`
	TimeInForce   string `json:"timeInForce,omitempty"`
	NewOrderResp  string `json:"newOrderRespType,omitempty"`
}

// ---------------------------------------------------------------------------
// Market Data
// ---------------------------------------------------------------------------

type TickerResponse struct {
	Symbol             string `json:"symbol"`
	PriceChange       string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	LastPrice         string `json:"lastPrice"`
	BidPrice          string `json:"bidPrice"`
	BidQty            string `json:"bidQty"`
	AskPrice          string `json:"askPrice"`
	AskQty            string `json:"askQty"`
	OpenPrice         string `json:"openPrice"`
	HighPrice         string `json:"highPrice"`
	LowPrice          string `json:"lowPrice"`
	Volume            string `json:"volume"`
	QuoteVolume       string `json:"quoteVolume"`
	OpenTime          int64  `json:"openTime"`
	CloseTime         int64  `json:"closeTime"`
	FirstID           int64  `json:"firstId"`
	LastID            int64  `json:"lastId"`
	Count             int64  `json:"count"`
}

type BookTickerResponse struct {
	Symbol    string `json:"symbol"`
	BidPrice  string `json:"bidPrice"`
	BidQty    string `json:"bidQty"`
	AskPrice  string `json:"askPrice"`
	AskQty    string `json:"askQty"`
}

type DepthResponse struct {
	LastUpdateID int64     `json:"lastUpdateId"`
	Bids        [][]string `json:"bids"`
	Asks        [][]string `json:"asks"`
}

type TradeResponse struct {
	ID              int64  `json:"id"`
	Symbol          string `json:"symbol"`
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
	Time            int64  `json:"time"`
	IsBuyerMaker    bool   `json:"isBuyerMaker"`
	IsBestMatch     bool   `json:"isBestMatch"`
}

type KlineResponse struct {
	OpenTime         int64  `json:"openTime"`
	Open             string `json:"open"`
	High             string `json:"high"`
	Low              string `json:"low"`
	Close            string `json:"close"`
	Volume           string `json:"volume"`
	CloseTime        int64  `json:"closeTime"`
	QuoteAssetVolume string `json:"quoteAssetVolume"`
	TradeCount       int64  `json:"tradeCount"`
}

// ---------------------------------------------------------------------------
// WebSocket Streams
// ---------------------------------------------------------------------------

type TickerStream struct {
	Symbol         string `json:"s"`
	LastPrice      string `json:"c"`
	BestBidPrice   string `json:"b"`
	BestBidQty     string `json:"B"`
	BestAskPrice   string `json:"a"`
	BestAskQty     string `json:"A"`
	OpenPrice      string `json:"o"`
	HighPrice      string `json:"h"`
	LowPrice       string `json:"l"`
	Volume         string `json:"v"`
	QuoteVolume    string `json:"q"`
	CloseTime      int64  `json:"E"`
	OpenTime       int64  `json:"O"`
}

type TradeStream struct {
	TradeID   int64  `json:"t"`
	Symbol    string `json:"s"`
	Price     string `json:"p"`
	Qty       string `json:"q"`
	Time      int64  `json:"T"`
	IsBuyerMaker bool `json:"m"`
}

type KlineStream struct {
	Symbol  string       `json:"s"`
	 Kline   KlineData   `json:"k"`
}

type KlineData struct {
	StartTime int64  `json:"t"`
	EndTime   int64  `json:"T"`
	Symbol    string `json:"s"`
	Interval  string `json:"i"`
	Open      string `json:"o"`
	Close     string `json:"c"`
	High      string `json:"h"`
	Low       string `json:"l"`
	Volume    string `json:"v"`
	IsClosed  bool   `json:"x"`
}

type DepthStream struct {
	UpdateID int64     `json:"u"`
	Symbol   string    `json:"s"`
	Bids     [][]string `json:"b"`
	Asks     [][]string `json:"a"`
}

type AggTradeStream struct {
	TradeID       int64  `json:"a"`
	Price         string `json:"p"`
	Quantity      string `json:"q"`
	FirstTradeID  int64  `json:"f"`
	LastTradeID   int64  `json:"l"`
	Time          int64  `json:"T"`
	IsBuyerMaker  bool   `json:"m"`
}

// ---------------------------------------------------------------------------
// User Data Stream
// ---------------------------------------------------------------------------

type UserDataStream struct {
	AccountUpdate    *AccountUpdate   `json:"accountUpdate,omitempty"`
	OrderUpdate      *OrderUpdate     `json:"orderTradeUpdate,omitempty"`
	BalanceUpdate    *BalanceUpdate   `json:"balanceUpdate,omitempty"`
	AccountTradeUpdate *TradeUpdate   `json:"accountTradeUpdate,omitempty"`
}

type AccountUpdate struct {
	balances []BalanceUpdate
}

type OrderUpdate struct {
	Symbol         string `json:"s"`
	ClientOrderID string `json:"c"`
	OrderID       int64  `json:"i"`
	Price         string `json:"p"`
	StopPrice     string `json:"P"`
	OrigQty       string `json:"q"`
	ExecutedQty   string `json:"z"`
	Status        string `json:"X"`
	Type          string `json:"o"`
	Side          string `json:"S"`
	UpdateTime    int64  `json:"T"`
}

type BalanceUpdate struct {
	Asset  string `json:"a"`
	Free   string `json:"f"`
	Locked string `json:"l"`
}

type TradeUpdate struct {
	Symbol         string `json:"s"`
	OrderID       int64  `json:"o"`
	TradeID       int64  `json:"t"`
	Price         string `json:"p"`
	Quantity      string `json:"q"`
	Commission    string `json:"n"`
	CommissionAsset string `json:"N"`
	Time          int64  `json:"T"`
	IsBuyer       bool   `json:"m"`
}

// ---------------------------------------------------------------------------
// Margin Trading
// ---------------------------------------------------------------------------

type MarginAccount struct {
	IsolateEnabled bool      `json:"isIsolatedEnabled"`
	Enabled       bool      `json:"enabled"`
	TotalAsset    string    `json:"totalAssetOfBtc"`
	TotalLiability string   `json:"totalLiabilityOfBtc"`
	TotalNetAsset string    `json:"totalNetAssetOfBtc"`
	TradeEnabled  bool      `json:"tradeEnabled"`
	TransferEnabled bool    `json:"transferEnabled"`
	MarginLevel   string    `json:"marginLevel"`
	MarginLevelOtm string   `json:"marginLevelOtm"`
}

type IsolatedMarginAccount struct {
	Symbol   string `json:"symbol"`
	Enabled  bool   `json:"enabled"`
	MarginLevel string `json:"marginLevel"`
}

// ---------------------------------------------------------------------------
// Futures
// ---------------------------------------------------------------------------

type FuturesAccount struct {
	TotalWalletBalance    string `json:"totalWalletBalance"`
	TotalUnrealizedProfit string `json:"totalUnrealizedProfit"`
	TotalMarginBalance   string `json:"totalMarginBalance"`
	TotalPositionInitialMargin string `json:"totalPositionInitialMargin"`
}

type PositionRisk struct {
	Symbol        string `json:"symbol"`
	PositionAmt   string `json:"positionAmt"`
	EntryPrice   string `json:"entryPrice"`
	UnrealizedProfit string `json:"unRealizedProfit"`
	LiquidationPrice string `json:"liquidationPrice"`
	Leverage      string `json:"leverage"`
	MarginType    string `json:"marginType"`
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

func (s *Symbol) GetMinNotional() string {
	for _, f := range s.Filters {
		if f.FilterType == "MIN_NOTIONAL" {
			return f.MinPrice
		}
	}
	return "0"
}

func (s *Symbol) GetLotSize() (minQty, maxQty, stepSize string) {
	for _, f := range s.Filters {
		if f.FilterType == "LOT_SIZE" {
			return f.MinQty, f.MaxQty, f.StepSize
		}
	}
	return "0", "0", "0"
}

func (s *Symbol) GetPriceFilter() (minPrice, maxPrice, tickSize string) {
	for _, f := range s.Filters {
		if f.FilterType == "PRICE_FILTER" {
			return f.MinPrice, f.MaxPrice, f.TickSize
		}
	}
	return "0", "0", "0"
}
