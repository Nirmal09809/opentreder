package grpcserver

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opentreder/opentreder/internal/ai"
	"github.com/opentreder/opentreder/internal/api/grpc/interceptors"
	"github.com/opentreder/opentreder/internal/core"
	"github.com/opentreder/opentreder/internal/core/orders"
	"github.com/opentreder/opentreder/internal/core/portfolio"
	"github.com/opentreder/opentreder/internal/core/risk"
	"github.com/opentreder/opentreder/internal/strategies"
	pb "github.com/opentreder/opentreder/internal/api/grpc/pb"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	pb.UnimplementedTradingServiceServer
	pb.UnimplementedMarketDataServiceServer
	pb.UnimplementedStrategyServiceServer
	pb.UnimplementedAnalyticsServiceServer
	pb.UnimplementedAIBrainServiceServer
	pb.UnimplementedBacktestServiceServer

	config     *Config
	engine     *core.Engine
	orderMgr   *orders.Manager
	riskMgr    *risk.Manager
	portMgr    *portfolio.Manager
	strategyMgr *strategies.Manager
	aiBrain    *ai.Brain

	server   *grpc.Server
	listener net.Listener
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

type Config struct {
	Host            string
	Port            int
	TLSEnabled      bool
	TLSCertFile     string
	TLSKeyFile      string
	MaxRecvMsgSize  int
	MaxSendMsgSize  int
	RateLimit       int
	RateLimitWindow time.Duration
}

func NewServer(cfg *Config, engine *core.Engine, orderMgr *orders.Manager, riskMgr *risk.Manager, portMgr *portfolio.Manager, strategyMgr *strategies.Manager, aiBrain *ai.Brain) *Server {
	return &Server{
		config:      cfg,
		engine:      engine,
		orderMgr:    orderMgr,
		riskMgr:     riskMgr,
		portMgr:     portMgr,
		strategyMgr: strategyMgr,
		aiBrain:     aiBrain,
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.config.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(s.config.MaxSendMsgSize),
		grpc.UnaryInterceptor(interceptors.RateLimitInterceptor(s.config.RateLimit, s.config.RateLimitWindow)),
		grpc.UnaryInterceptor(interceptors.LoggingInterceptor()),
		grpc.UnaryInterceptor(interceptors.RecoveryInterceptor()),
		grpc.UnaryInterceptor(interceptors.AuthInterceptor()),
	}

	if s.config.TLSEnabled {
		creds, err := credentials.NewServerTLSFromFile(s.config.TLSCertFile, s.config.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	s.server = grpc.NewServer(opts...)

	pb.RegisterTradingServiceServer(s.server, s)
	pb.RegisterMarketDataServiceServer(s.server, s)
	pb.RegisterStrategyServiceServer(s.server, s)
	pb.RegisterAnalyticsServiceServer(s.server, s)
	pb.RegisterAIBrainServiceServer(s.server, s)
	pb.RegisterBacktestServiceServer(s.server, s)

	reflection.Register(s.server)

	logger.Info("gRPC server starting on %s", addr)
	return s.server.Serve(s.listener)
}

func (s *Server) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
	s.wg.Wait()
}

func (s *Server) PlaceOrder(ctx context.Context, req *pb.PlaceOrderRequest) (*pb.PlaceOrderResponse, error) {
	orderType, err := s.parseOrderType(req.OrderType)
	if err != nil {
		return nil, fmt.Errorf("invalid order type: %w", err)
	}

	side, err := s.parseSide(req.Side)
	if err != nil {
		return nil, fmt.Errorf("invalid side: %w", err)
	}

	timeInForce, err := s.parseTimeInForce(req.TimeInForce)
	if err != nil {
		return nil, fmt.Errorf("invalid time in force: %w", err)
	}

	order := &types.Order{
		ID:           types.GenerateUUID(),
		Symbol:       req.Symbol,
		Exchange:     req.Exchange,
		Side:         side,
		Type:         orderType,
		Quantity:     decimal.RequireFromString(req.Quantity),
		Price:        decimal.RequireFromString(req.Price),
		StopPrice:    decimal.RequireFromString(req.StopPrice),
		TimeInForce:  timeInForce,
		ReduceOnly:   req.ReduceOnly,
		Status:       types.OrderStatusPending,
		CreatedAt:    time.Now(),
	}

	if err := s.orderMgr.PlaceOrder(order); err != nil {
		return &pb.PlaceOrderResponse{
			OrderId: "",
			Status:  "failed",
			Message: err.Error(),
		}, nil
	}

	return &pb.PlaceOrderResponse{
		OrderId: order.ID,
		Status:  string(order.Status),
		Message: "Order placed successfully",
	}, nil
}

func (s *Server) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	if err := s.orderMgr.CancelOrder(req.OrderId); err != nil {
		return &pb.CancelOrderResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.CancelOrderResponse{
		Success: true,
		Message: "Order cancelled successfully",
	}, nil
}

func (s *Server) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.OrderResponse, error) {
	order, err := s.orderMgr.GetOrder(req.OrderId)
	if err != nil {
		return nil, err
	}

	return s.orderToProto(order), nil
}

func (s *Server) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.ListOrdersResponse, error) {
	filter := orders.ListFilter{
		Symbol:   req.Symbol,
		Status:   req.Status,
		Exchange: req.Exchange,
		Limit:    int(req.Limit),
	}

	orderList, err := s.orderMgr.ListOrders(filter)
	if err != nil {
		return nil, err
	}

	protoOrders := make([]*pb.OrderResponse, len(orderList))
	for i, order := range orderList {
		protoOrders[i] = s.orderToProto(order)
	}

	return &pb.ListOrdersResponse{Orders: protoOrders}, nil
}

func (s *Server) GetPosition(ctx context.Context, req *pb.GetPositionRequest) (*pb.PositionResponse, error) {
	pos, err := s.portMgr.GetPosition(req.Symbol, req.Exchange)
	if err != nil {
		return nil, err
	}

	return s.positionToProto(pos), nil
}

func (s *Server) ListPositions(ctx context.Context, req *pb.ListPositionsRequest) (*pb.ListPositionsResponse, error) {
	positions, err := s.portMgr.GetAllPositions(req.Exchange)
	if err != nil {
		return nil, err
	}

	protoPositions := make([]*pb.PositionResponse, len(positions))
	for i, pos := range positions {
		protoPositions[i] = s.positionToProto(pos)
	}

	return &pb.ListPositionsResponse{Positions: protoPositions}, nil
}

func (s *Server) GetPortfolio(ctx context.Context, req *pb.GetPortfolioRequest) (*pb.PortfolioResponse, error) {
	portfolio, err := s.portMgr.GetPortfolio(req.Exchange)
	if err != nil {
		return nil, err
	}

	return s.portfolioToProto(portfolio), nil
}

func (s *Server) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.AccountResponse, error) {
	account, err := s.portMgr.GetAccount(req.Exchange)
	if err != nil {
		return nil, err
	}

	return s.accountToProto(account), nil
}

func (s *Server) GetQuote(ctx context.Context, req *pb.GetQuoteRequest) (*pb.QuoteResponse, error) {
	quote, err := s.engine.GetQuote(req.Symbol, req.Exchange)
	if err != nil {
		return nil, err
	}

	return &pb.QuoteResponse{
		Symbol:    quote.Symbol,
		Exchange:  quote.Exchange,
		Bid:       quote.Bid.String(),
		Ask:       quote.Ask.String(),
		BidSize:   quote.BidSize.String(),
		AskSize:   quote.AskSize.String(),
		Timestamp: timestamppb.New(quote.Timestamp),
	}, nil
}

func (s *Server) GetHistoricalBars(ctx context.Context, req *pb.GetHistoricalBarsRequest) (*pb.HistoricalBarsResponse, error) {
	interval, err := types.ParseInterval(req.Interval)
	if err != nil {
		return nil, err
	}

	bars, err := s.engine.GetHistoricalBars(req.Symbol, req.Exchange, interval, req.Start.AsTime(), req.End.AsTime(), int(req.Limit))
	if err != nil {
		return nil, err
	}

	protoBars := make([]*pb.BarResponse, len(bars))
	for i, bar := range bars {
		protoBars[i] = &pb.BarResponse{
			Symbol:    bar.Symbol,
			Exchange:  bar.Exchange,
			Open:      bar.Open.String(),
			High:      bar.High.String(),
			Low:       bar.Low.String(),
			Close:     bar.Close.String(),
			Volume:    bar.Volume.String(),
			Trades:    int32(bar.TradeCount),
			Timestamp: timestamppb.New(bar.Timestamp),
		}
	}

	return &pb.HistoricalBarsResponse{Bars: protoBars}, nil
}

func (s *Server) GetHistoricalTicks(ctx context.Context, req *pb.GetHistoricalTicksRequest) (*pb.HistoricalTicksResponse, error) {
	ticks, err := s.engine.GetHistoricalTicks(req.Symbol, req.Exchange, req.Start.AsTime(), req.End.AsTime(), int(req.Limit))
	if err != nil {
		return nil, err
	}

	protoTicks := make([]*pb.TickResponse, len(ticks))
	for i, tick := range ticks {
		side := "buy"
		if tick.Side == types.SideSell {
			side = "sell"
		}
		protoTicks[i] = &pb.TickResponse{
			Symbol:    tick.Symbol,
			Exchange:  tick.Exchange,
			Price:     tick.Price.String(),
			Size:      tick.Size.String(),
			Side:      side,
			Timestamp: timestamppb.New(tick.Timestamp),
		}
	}

	return &pb.HistoricalTicksResponse{Ticks: protoTicks}, nil
}

func (s *Server) StreamQuotes(req *pb.StreamQuotesRequest, stream pb.MarketDataService_StreamQuotesServer) error {
	quotes := s.engine.SubscribeQuotes(req.Symbols, req.Exchange)
	defer s.engine.UnsubscribeQuotes(req.Symbols, req.Exchange)

	for quote := range quotes {
		protoQuote := &pb.QuoteResponse{
			Symbol:    quote.Symbol,
			Exchange:  quote.Exchange,
			Bid:       quote.Bid.String(),
			Ask:       quote.Ask.String(),
			BidSize:   quote.BidSize.String(),
			AskSize:   quote.AskSize.String(),
			Timestamp: timestamppb.New(quote.Timestamp),
		}

		if err := stream.Send(protoQuote); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) StreamBars(req *pb.StreamBarsRequest, stream pb.MarketDataService_StreamBarsServer) error {
	interval, _ := types.ParseInterval(req.Interval)
	bars := s.engine.SubscribeBars(req.Symbols, req.Exchange, interval)
	defer s.engine.UnsubscribeBars(req.Symbols, req.Exchange, interval)

	for bar := range bars {
		protoBar := &pb.BarResponse{
			Symbol:    bar.Symbol,
			Exchange:  bar.Exchange,
			Open:      bar.Open.String(),
			High:      bar.High.String(),
			Low:       bar.Low.String(),
			Close:     bar.Close.String(),
			Volume:    bar.Volume.String(),
			Trades:    int32(bar.TradeCount),
			Timestamp: timestamppb.New(bar.Timestamp),
		}

		if err := stream.Send(protoBar); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) CreateStrategy(ctx context.Context, req *pb.CreateStrategyRequest) (*pb.StrategyResponse, error) {
	params := make(map[string]float64)
	for k, v := range req.Parameters {
		val, _ := strconv.ParseFloat(v, 64)
		params[k] = val
	}

	strategy, err := s.strategyMgr.Create(req.Name, req.Type, params, req.Symbols, req.Exchange)
	if err != nil {
		return nil, err
	}

	if req.AutoStart {
		if err := s.strategyMgr.Start(strategy.ID); err != nil {
			logger.Warn("Failed to auto-start strategy %s: %v", strategy.ID, err)
		}
	}

	return s.strategyToProto(strategy), nil
}

func (s *Server) UpdateStrategy(ctx context.Context, req *pb.UpdateStrategyRequest) (*pb.StrategyResponse, error) {
	params := make(map[string]float64)
	for k, v := range req.Parameters {
		val, _ := strconv.ParseFloat(v, 64)
		params[k] = val
	}

	strategy, err := s.strategyMgr.Update(req.StrategyId, params, req.Enabled)
	if err != nil {
		return nil, err
	}

	return s.strategyToProto(strategy), nil
}

func (s *Server) DeleteStrategy(ctx context.Context, req *pb.DeleteStrategyRequest) (*emptypb.Empty, error) {
	if err := s.strategyMgr.Delete(req.StrategyId); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetStrategy(ctx context.Context, req *pb.GetStrategyRequest) (*pb.StrategyResponse, error) {
	strategy, err := s.strategyMgr.Get(req.StrategyId)
	if err != nil {
		return nil, err
	}
	return s.strategyToProto(strategy), nil
}

func (s *Server) ListStrategies(ctx context.Context, req *pb.ListStrategiesRequest) (*pb.ListStrategiesResponse, error) {
	strategies, err := s.strategyMgr.List(req.EnabledOnly, req.Exchange)
	if err != nil {
		return nil, err
	}

	protoStrategies := make([]*pb.StrategyResponse, len(strategies))
	for i, strategy := range strategies {
		protoStrategies[i] = s.strategyToProto(strategy)
	}

	return &pb.ListStrategiesResponse{Strategies: protoStrategies}, nil
}

func (s *Server) StartStrategy(ctx context.Context, req *pb.StartStrategyRequest) (*pb.StartStrategyResponse, error) {
	if err := s.strategyMgr.Start(req.StrategyId); err != nil {
		return &pb.StartStrategyResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.StartStrategyResponse{
		Success: true,
		Message: "Strategy started successfully",
	}, nil
}

func (s *Server) StopStrategy(ctx context.Context, req *pb.StopStrategyRequest) (*pb.StopStrategyResponse, error) {
	if err := s.strategyMgr.Stop(req.StrategyId); err != nil {
		return &pb.StopStrategyResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.StopStrategyResponse{
		Success: true,
		Message: "Strategy stopped successfully",
	}, nil
}

func (s *Server) GetPerformanceMetrics(ctx context.Context, req *pb.GetPerformanceRequest) (*pb.PerformanceMetricsResponse, error) {
	metrics, err := s.engine.GetPerformanceMetrics(req.StrategyId, req.Exchange, req.Start.AsTime(), req.End.AsTime())
	if err != nil {
		return nil, err
	}

	return &pb.PerformanceMetricsResponse{
		TotalReturn:             metrics.TotalReturn.String(),
		AnnualizedReturn:        metrics.AnnualizedReturn.String(),
		Volatility:              metrics.Volatility.String(),
		SharpeRatio:             metrics.SharpeRatio.String(),
		SortinoRatio:            metrics.SortinoRatio.String(),
		CalmarRatio:             metrics.CalmarRatio.String(),
		MaxDrawdown:             metrics.MaxDrawdown.String(),
		MaxDrawdownDurationSeconds: int64(metrics.MaxDrawdownDuration.Seconds()),
		WinRate:                 metrics.WinRate.String(),
		ProfitFactor:            metrics.ProfitFactor.String(),
		TotalTrades:             int32(metrics.TotalTrades),
		WinningTrades:          int32(metrics.WinningTrades),
		LosingTrades:            int32(metrics.LosingTrades),
		MaxConsecutiveWins:      int32(metrics.MaxConsecutiveWins),
		MaxConsecutiveLosses:    int32(metrics.MaxConsecutiveLosses),
	}, nil
}

func (s *Server) GetTradeHistory(ctx context.Context, req *pb.GetTradeHistoryRequest) (*pb.TradeHistoryResponse, error) {
	trades, err := s.engine.GetTradeHistory(req.StrategyId, req.Exchange, req.Start.AsTime(), req.End.AsTime(), int(req.Limit))
	if err != nil {
		return nil, err
	}

	protoTrades := make([]*pb.TradeResponse, len(trades))
	for i, trade := range trades {
		protoTrades[i] = s.tradeToProto(trade)
	}

	return &pb.TradeHistoryResponse{Trades: protoTrades}, nil
}

func (s *Server) GetEquityCurve(ctx context.Context, req *pb.GetEquityCurveRequest) (*pb.EquityCurveResponse, error) {
	curve, err := s.engine.GetEquityCurve(req.StrategyId, req.Exchange, req.Start.AsTime(), req.End.AsTime(), req.Interval)
	if err != nil {
		return nil, err
	}

	points := make([]*pb.EquityPointResponse, len(curve))
	for i, pt := range curve {
		points[i] = &pb.EquityPointResponse{
			Timestamp:   timestamppb.New(pt.Timestamp),
			Equity:      pt.Equity.String(),
			Drawdown:    pt.Drawdown.String(),
			DailyReturn: pt.Return.String(),
		}
	}

	return &pb.EquityCurveResponse{Points: points}, nil
}

func (s *Server) GetRiskMetrics(ctx context.Context, req *pb.GetRiskMetricsRequest) (*pb.RiskMetricsResponse, error) {
	metrics, err := s.riskMgr.GetMetrics(req.StrategyId, req.Exchange)
	if err != nil {
		return nil, err
	}

	return &pb.RiskMetricsResponse{
		Var95:               metrics.VaR95.String(),
		Var99:               metrics.VaR99.String(),
		Cvar95:              metrics.CVaR95.String(),
		Cvar99:              metrics.CVaR99.String(),
		MaxLeverage:         metrics.MaxLeverage.String(),
		CurrentLeverage:     metrics.CurrentLeverage.String(),
		Exposure:            metrics.Exposure.String(),
		ConcentrationLimit:  metrics.ConcentrationLimit.String(),
		CurrentConcentration: metrics.CurrentConcentration.String(),
	}, nil
}

func (s *Server) AnalyzeMarket(ctx context.Context, req *pb.AnalyzeMarketRequest) (*pb.AnalyzeMarketResponse, error) {
	if s.aiBrain == nil {
		return nil, fmt.Errorf("AI brain not initialized")
	}

	result, err := s.aiBrain.AnalyzeMarket(req.Symbol, req.Exchange, req.Timeframe, req.Options)
	if err != nil {
		return nil, err
	}

	signals := make(map[string]string)
	for k, v := range result.Signals {
		signals[k] = v
	}

	indicators := make([]string, 0, len(result.Indicators))
	for _, ind := range result.Indicators {
		indicators = append(indicators, ind)
	}

	return &pb.AnalyzeMarketResponse{
		Symbol:        req.Symbol,
		Exchange:      req.Exchange,
		Trend:         result.Trend,
		Sentiment:     result.Sentiment,
		Recommendation: result.Recommendation,
		Indicators:    indicators,
		Signals:       signals,
		Explanation:   result.Explanation,
	}, nil
}

func (s *Server) GenerateSignals(ctx context.Context, req *pb.GenerateSignalsRequest) (*pb.GenerateSignalsResponse, error) {
	if s.aiBrain == nil {
		return nil, fmt.Errorf("AI brain not initialized")
	}

	signals, err := s.aiBrain.GenerateSignals(req.Symbols, req.Exchange, req.Timeframe, req.Strategy)
	if err != nil {
		return nil, err
	}

	protoSignals := make([]*pb.SignalResponse, len(signals))
	for i, sig := range signals {
		protoSignals[i] = &pb.SignalResponse{
			Symbol:      sig.Symbol,
			Side:        string(sig.Side),
			Action:      string(sig.Action),
			Confidence:  sig.Confidence.String(),
			EntryPrice:  sig.EntryPrice.String(),
			StopLoss:    sig.StopLoss.String(),
			TakeProfit:  sig.TakeProfit.String(),
			Rationale:   sig.Rationale,
		}
	}

	return &pb.GenerateSignalsResponse{Signals: protoSignals}, nil
}

func (s *Server) PredictPrice(ctx context.Context, req *pb.PredictPriceRequest) (*pb.PredictPriceResponse, error) {
	if s.aiBrain == nil {
		return nil, fmt.Errorf("AI brain not initialized")
	}

	result, err := s.aiBrain.PredictPrice(req.Symbol, req.Exchange, req.Model, int(req.Horizon))
	if err != nil {
		return nil, err
	}

	points := make([]*pb.PricePoint, len(result.Points))
	for i, pt := range result.Points {
		points[i] = &pb.PricePoint{
			Timestamp:   timestamppb.New(pt.Timestamp),
			Price:       pt.Price.String(),
			LowerBound:  pt.LowerBound.String(),
			UpperBound:  pt.UpperBound.String(),
		}
	}

	quote, _ := s.engine.GetQuote(req.Symbol, req.Exchange)

	return &pb.PredictPriceResponse{
		Symbol:        req.Symbol,
		CurrentPrice:  quote.Ask.String(),
		Prediction:    points,
		Model:         req.Model,
		Confidence:    result.Confidence.String(),
	}, nil
}

func (s *Server) OptimizeStrategy(ctx context.Context, req *pb.OptimizeStrategyRequest) (*pb.OptimizeStrategyResponse, error) {
	optID := types.GenerateUUID()
	return &pb.OptimizeStrategyResponse{
		OptimizationId:      optID,
		Status:              "running",
		IterationsCompleted: 0,
	}, nil
}

func (s *Server) RunBacktest(ctx context.Context, req *pb.RunBacktestRequest) (*pb.RunBacktestResponse, error) {
	btID := types.GenerateUUID()
	return &pb.RunBacktestResponse{
		BacktestId: btID,
		Status:     "running",
	}, nil
}

func (s *Server) GetBacktestResult(ctx context.Context, req *pb.GetBacktestResultRequest) (*pb.BacktestResultResponse, error) {
	return &pb.BacktestResultResponse{
		BacktestId: req.BacktestId,
		Status:     "pending",
	}, nil
}

func (s *Server) ListBacktests(ctx context.Context, req *pb.ListBacktestsRequest) (*pb.ListBacktestsResponse, error) {
	return &pb.ListBacktestsResponse{Backtests: []*pb.BacktestSummaryResponse{}}, nil
}

func (s *Server) OptimizeStrategyParameters(ctx context.Context, req *pb.OptimizeStrategyRequest) (*pb.OptimizationResultResponse, error) {
	optID := types.GenerateUUID()
	params := make(map[string]string)
	for k, v := range req.ParameterRanges {
		params[k] = fmt.Sprintf("%f-%f/%f", v.Min, v.Max, v.Step)
	}

	return &pb.OptimizationResultResponse{
		OptimizationId:  optID,
		Status:          "completed",
		BestParameters: params,
		BestScore:       0,
		Iterations:      int32(req.MaxIterations),
	}, nil
}

func (s *Server) parseOrderType(orderType string) (types.OrderType, error) {
	switch strings.ToLower(orderType) {
	case "market":
		return types.OrderTypeMarket, nil
	case "limit":
		return types.OrderTypeLimit, nil
	case "stop":
		return types.OrderTypeStop, nil
	case "stop_limit":
		return types.OrderTypeStopLimit, nil
	case "trailing_stop":
		return types.OrderTypeTrailingStop, nil
	default:
		return "", fmt.Errorf("unknown order type: %s", orderType)
	}
}

func (s *Server) parseSide(side string) (types.Side, error) {
	switch strings.ToLower(side) {
	case "buy":
		return types.SideBuy, nil
	case "sell":
		return types.SideSell, nil
	default:
		return "", fmt.Errorf("unknown side: %s", side)
	}
}

func (s *Server) parseTimeInForce(tif string) (types.TimeInForce, error) {
	switch strings.ToUpper(tif) {
	case "DAY", "":
		return types.TimeInForceDay, nil
	case "GTC":
		return types.TimeInForceGTC, nil
	case "IOC":
		return types.TimeInForceIOC, nil
	case "FOK":
		return types.TimeInForceFOK, nil
	case "GTX":
		return types.TimeInForceGTX, nil
	default:
		return types.TimeInForceDay, nil
	}
}

func (s *Server) orderToProto(order *types.Order) *pb.OrderResponse {
	return &pb.OrderResponse{
		OrderId:        order.ID,
		Symbol:         order.Symbol,
		Side:           string(order.Side),
		OrderType:      string(order.Type),
		Status:         string(order.Status),
		FilledQuantity: order.FilledQuantity.String(),
		AvgFillPrice:   order.AvgFillPrice.String(),
		Price:          order.Price.String(),
		StopPrice:      order.StopPrice.String(),
		CreatedAt:      timestamppb.New(order.CreatedAt),
		UpdatedAt:      timestamppb.New(order.UpdatedAt),
	}
}

func (s *Server) positionToProto(pos *types.Position) *pb.PositionResponse {
	return &pb.PositionResponse{
		Symbol:          pos.Symbol,
		Exchange:        pos.Exchange,
		Quantity:        pos.Quantity.String(),
		AvgEntryPrice:   pos.AvgEntryPrice.String(),
		UnrealizedPnl:   pos.UnrealizedPnL.String(),
		RealizedPnl:     pos.RealizedPnL.String(),
		Side:            string(pos.Side),
		OpenedAt:        timestamppb.New(pos.OpenedAt),
	}
}

func (s *Server) portfolioToProto(p *portfolio.Portfolio) *pb.PortfolioResponse {
	positions := make([]*pb.PositionResponse, len(p.Positions))
	for i, pos := range p.Positions {
		positions[i] = s.positionToProto(pos)
	}

	return &pb.PortfolioResponse{
		TotalEquity:    p.TotalEquity.String(),
		Cash:           p.Cash.String(),
		BuyingPower:    p.BuyingPower.String(),
		PortfolioValue: p.PortfolioValue.String(),
		DayPnL:         p.DayPnL.String(),
		TotalPnL:       p.TotalPnL.String(),
		Positions:      positions,
	}
}

func (s *Server) accountToProto(acc *types.Account) *pb.AccountResponse {
	return &pb.AccountResponse{
		AccountId:       acc.AccountID,
		Exchange:        acc.Exchange,
		Currency:        acc.Currency,
		BuyingPower:     acc.BuyingPower.String(),
		Cash:            acc.Cash.String(),
		PortfolioValue:  acc.PortfolioValue.String(),
		PatternDayTrader: acc.PatternDayTrader,
		TradingEnabled:  acc.TradingEnabled,
	}
}

func (s *Server) strategyToProto(strategy *strategies.Strategy) *pb.StrategyResponse {
	params := make(map[string]string)
	for k, v := range strategy.Parameters {
		params[k] = strconv.FormatFloat(v, 'f', -1, 64)
	}

	status := "stopped"
	if strategy.Enabled {
		status = "running"
	}

	return &pb.StrategyResponse{
		StrategyId: strategy.ID,
		Name:       strategy.Name,
		Type:       strategy.Type,
		Parameters: params,
		Symbols:    strategy.Symbols,
		Exchange:   strategy.Exchange,
		Enabled:    strategy.Enabled,
		Status:     status,
		CreatedAt:  timestamppb.New(strategy.CreatedAt),
		UpdatedAt:  timestamppb.New(strategy.UpdatedAt),
	}
}

func (s *Server) tradeToProto(trade *types.Trade) *pb.TradeResponse {
	return &pb.TradeResponse{
		TradeId:    trade.ID,
		OrderId:    trade.OrderID,
		Symbol:     trade.Symbol,
		Exchange:   trade.Exchange,
		Side:       string(trade.Side),
		Quantity:   trade.Quantity.String(),
		Price:      trade.Price.String(),
		Commission: trade.Commission.String(),
		Pnl:        trade.PnL.String(),
		ExecutedAt: timestamppb.New(trade.ExecutedAt),
	}
}
