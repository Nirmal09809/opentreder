package pb

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UnimplementedTradingServiceServer struct{}

func (UnimplementedTradingServiceServer) SubmitOrder(context.Context, *SubmitOrderRequest) (*SubmitOrderResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SubmitOrder not implemented")
}

type SubmitOrderRequest struct {
	ClientOrderID string `json:"client_order_id"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	Quantity      string `json:"quantity"`
	Price         string `json:"price"`
	StopPrice     string `json:"stop_price"`
}

type SubmitOrderResponse struct {
	OrderID       string `json:"order_id"`
	ClientOrderID string `json:"client_order_id"`
	Status        string `json:"status"`
	CreatedAt     *timestamppb.Timestamp `json:"created_at"`
}

type UnimplementedMarketDataServiceServer struct{}

type UnimplementedStrategyServiceServer struct{}

type UnimplementedAnalyticsServiceServer struct{}

type UnimplementedAIBrainServiceServer struct{}

type UnimplementedBacktestServiceServer struct{}

type TradingServiceServer interface {
	SubmitOrder(context.Context, *SubmitOrderRequest) (*SubmitOrderResponse, error)
	GetOrder(context.Context, *GetOrderRequest) (*GetOrderResponse, error)
	CancelOrder(context.Context, *CancelOrderRequest) (*CancelOrderResponse, error)
	GetOpenOrders(context.Context, *GetOpenOrdersRequest) (*GetOpenOrdersResponse, error)
	GetPositions(context.Context, *emptypb.Empty) (*GetPositionsResponse, error)
	GetPortfolio(context.Context, *emptypb.Empty) (*GetPortfolioResponse, error)
}

type GetOrderRequest struct {
	OrderID string `json:"order_id"`
}

type GetOrderResponse struct {
	OrderID       string `json:"order_id"`
	ClientOrderID string `json:"client_order_id"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	Quantity      string `json:"quantity"`
	Price         string `json:"price"`
	StopPrice     string `json:"stop_price"`
	Status        string `json:"status"`
	FilledQty     string `json:"filled_qty"`
	AvgFillPrice  string `json:"avg_fill_price"`
	CreatedAt     *timestamppb.Timestamp `json:"created_at"`
	UpdatedAt     *timestamppb.Timestamp `json:"updated_at"`
}

type CancelOrderRequest struct {
	OrderID string `json:"order_id"`
}

type CancelOrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type GetOpenOrdersRequest struct {
	Symbol string `json:"symbol"`
}

type GetOpenOrdersResponse struct {
	Orders []*GetOrderResponse `json:"orders"`
}

type Position struct {
	Symbol        string `json:"symbol"`
	Quantity      string `json:"quantity"`
	AvgEntryPrice string `json:"avg_entry_price"`
	MarketValue   string `json:"market_value"`
	UnrealizedPnL string `json:"unrealized_pnl"`
}

type GetPositionsResponse struct {
	Positions []*Position `json:"positions"`
}

type Portfolio struct {
	TotalValue     string `json:"total_value"`
	CashBalance    string `json:"cash_balance"`
	EquityValue    string `json:"equity_value"`
	UnrealizedPnL  string `json:"unrealized_pnl"`
	RealizedPnL    string `json:"realized_pnl"`
	BuyingPower    string `json:"buying_power"`
	DayPnL         string `json:"day_pnl"`
	DayReturn      string `json:"day_return"`
}

type GetPortfolioResponse struct {
	Portfolio *Portfolio `json:"portfolio"`
}

func RegisterTradingServiceServer(s *grpc.Server, srv TradingServiceServer) {}
