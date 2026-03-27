package grpcserver

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opentreder/opentreder/internal/core"
	"github.com/opentreder/opentreder/internal/core/orders"
	"github.com/opentreder/opentreder/internal/core/portfolio"
	"github.com/opentreder/opentreder/internal/core/risk"
	"github.com/opentreder/opentreder/internal/strategies"
	pb "github.com/opentreder/opentreder/internal/api/grpc/pb"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Config struct {
	Host            string
	Port            int
	TLSEnabled      bool
	TLSCertFile     string
	TLSKeyFile      string
	RateLimit       int
	RateLimitWindow time.Duration
}

type Server struct {
	pb.UnimplementedTradingServiceServer

	config      *Config
	engine     *core.Engine
	orderMgr   *orders.Manager
	riskMgr    *risk.Manager
	portMgr    *portfolio.Manager
	strategyMgr *strategies.StrategyManager

	server   *grpc.Server
	listener net.Listener
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

func NewServer(cfg *Config, engine *core.Engine, orderMgr *orders.Manager, riskMgr *risk.Manager, portMgr *portfolio.Manager, strategyMgr *strategies.StrategyManager) *Server {
	return &Server{
		config:      cfg,
		engine:      engine,
		orderMgr:    orderMgr,
		riskMgr:     riskMgr,
		portMgr:     portMgr,
		strategyMgr: strategyMgr,
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	opts := []grpc.ServerOption{}
	s.server = grpc.NewServer(opts...)
	pb.RegisterTradingServiceServer(s.server, s)

	logger.Info("gRPC server starting on %s", addr)
	return s.server.Serve(s.listener)
}

func (s *Server) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}

func (s *Server) SubmitOrder(ctx context.Context, req *pb.SubmitOrderRequest) (*pb.SubmitOrderResponse, error) {
	orderType := types.OrderType(req.Type)
	side := types.OrderSide(req.Side)

	orderID := uuid.New()
	order := &types.Order{
		ID:        orderID,
		Symbol:    req.Symbol,
		Exchange:  "binance",
		Side:      side,
		Type:      orderType,
		Quantity:  decimal.RequireFromString(req.Quantity),
		Price:     decimal.RequireFromString(req.Price),
		StopPrice: decimal.RequireFromString(req.StopPrice),
		Status:    types.OrderStatusPending,
		CreatedAt: time.Now(),
	}

	return &pb.SubmitOrderResponse{
		OrderID:       orderID.String(),
		ClientOrderID: order.ClientOrderID,
		Status:        string(order.Status),
	}, nil
}

func (s *Server) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	return &pb.GetOrderResponse{
		OrderID: req.OrderID,
	}, nil
}

func (s *Server) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	return &pb.CancelOrderResponse{
		OrderID: req.OrderID,
		Status:  "cancelled",
	}, nil
}

func (s *Server) GetOpenOrders(ctx context.Context, req *pb.GetOpenOrdersRequest) (*pb.GetOpenOrdersResponse, error) {
	return &pb.GetOpenOrdersResponse{}, nil
}

func (s *Server) GetPositions(ctx context.Context, req *emptypb.Empty) (*pb.GetPositionsResponse, error) {
	return &pb.GetPositionsResponse{}, nil
}

func (s *Server) GetPortfolio(ctx context.Context, req *emptypb.Empty) (*pb.GetPortfolioResponse, error) {
	return &pb.GetPortfolioResponse{}, nil
}
