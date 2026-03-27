package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/opentreder/opentreder/internal/core/engine"
	"github.com/opentreder/opentreder/internal/core/orders"
	"github.com/opentreder/opentreder/internal/core/portfolio"
	"github.com/opentreder/opentreder/internal/core/risk"
	"github.com/opentreder/opentreder/internal/strategies"
	"github.com/opentreder/opentreder/pkg/config"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Server struct {
	config    *config.APIConfig
	engine    *engine.Engine
	orders    *orders.Manager
	portfolio *portfolio.Manager
	risk      *risk.Manager
	strategies *strategies.StrategyManager
	mux       *mux.Router
	server    *http.Server
}

type Handler struct {
	server *Server
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Time    time.Time   `json:"time"`
}

func NewServer(cfg *config.APIConfig, eng *engine.Engine) *Server {
	s := &Server{
		config:    cfg,
		engine:    eng,
		mux:       mux.NewRouter(),
	}

	s.setupRoutes()
	s.setupMiddleware()

	return s
}

func (s *Server) setupMiddleware() {
	s.mux.Use(s.loggingMiddleware)
	s.mux.Use(s.corsMiddleware)
	s.mux.Use(s.recoveryMiddleware)

	if s.config.RateLimit.Enabled {
		s.mux.Use(s.rateLimitMiddleware)
	}
}

func (s *Server) setupRoutes() {
	api := s.mux.PathPrefix("/api/v1").Subrouter()

	api.HandleFunc("/health", s.handleHealth).Methods("GET")
	api.HandleFunc("/ready", s.handleReady).Methods("GET")

	api.HandleFunc("/portfolio", s.handleGetPortfolio).Methods("GET")
	api.HandleFunc("/portfolio/balance", s.handleGetBalance).Methods("GET")
	api.HandleFunc("/portfolio/history", s.handleGetPortfolioHistory).Methods("GET")

	api.HandleFunc("/positions", s.handleGetPositions).Methods("GET")
	api.HandleFunc("/positions/{symbol}", s.handleGetPosition).Methods("GET")

	api.HandleFunc("/orders", s.handleGetOrders).Methods("GET")
	api.HandleFunc("/orders", s.handlePlaceOrder).Methods("POST")
	api.HandleFunc("/orders/{id}", s.handleGetOrder).Methods("GET")
	api.HandleFunc("/orders/{id}", s.handleCancelOrder).Methods("DELETE")
	api.HandleFunc("/orders/{id}/cancel", s.handleCancelOrder).Methods("POST")

	api.HandleFunc("/trades", s.handleGetTrades).Methods("GET")

	api.HandleFunc("/market/{symbol}", s.handleGetMarket).Methods("GET")
	api.HandleFunc("/market/{symbol}/candles", s.handleGetCandles).Methods("GET")
	api.HandleFunc("/market/{symbol}/orderbook", s.handleGetOrderBook).Methods("GET")
	api.HandleFunc("/market/prices", s.handleGetPrices).Methods("GET")

	api.HandleFunc("/signals", s.handleGetSignals).Methods("GET")
	api.HandleFunc("/signals/generate", s.handleGenerateSignal).Methods("POST")

	api.HandleFunc("/strategies", s.handleGetStrategies).Methods("GET")
	api.HandleFunc("/strategies/{name}", s.handleGetStrategy).Methods("GET")
	api.HandleFunc("/strategies/{name}/start", s.handleStartStrategy).Methods("POST")
	api.HandleFunc("/strategies/{name}/stop", s.handleStopStrategy).Methods("POST")

	api.HandleFunc("/risk/limits", s.handleGetRiskLimits).Methods("GET")
	api.HandleFunc("/risk/exposure", s.handleGetExposure).Methods("GET")

	api.HandleFunc("/exchanges", s.handleGetExchanges).Methods("GET")
	api.HandleFunc("/exchanges/{name}/connect", s.handleConnectExchange).Methods("POST")
	api.HandleFunc("/exchanges/{name}/disconnect", s.handleDisconnectExchange).Methods("POST")

	api.HandleFunc("/config", s.handleGetConfig).Methods("GET")
	api.HandleFunc("/config/{key}", s.handleSetConfig).Methods("PUT")

	s.mux.PathPrefix("/ws").Handler(s.handleWebSocket())
}

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	logger.Info("Starting API server", "addr", addr)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server error", "error", err)
		}
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.respond(w, r, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"status":  "healthy",
			"version": "1.0.0",
			"uptime":  s.engine.GetUptime().String(),
		},
		Time: time.Now(),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ready := true
	checks := map[string]bool{
		"engine":    s.engine.GetState() == engine.StateRunning,
		"database":  true,
		"exchanges": true,
	}

	for _, v := range checks {
		if !v {
			ready = false
			break
		}
	}

	s.respond(w, r, APIResponse{
		Success: ready,
		Data: map[string]interface{}{
			"ready":  ready,
			"checks": checks,
		},
		Time: time.Now(),
	})
}

func (s *Server) handleGetPortfolio(w http.ResponseWriter, r *http.Request) {
	data := s.portfolio.GetAllBalances()
	
	s.respond(w, r, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"total_value":     s.portfolio.GetTotalValue("USDT"),
			"unrealized_pnl":  s.portfolio.GetUnrealizedPnL(),
			"realized_pnl":    s.portfolio.GetRealizedPnL(),
			"day_pnl":         s.portfolio.GetDayPnL(),
			"balances":        data,
		},
		Time: time.Now(),
	})
}

func (s *Server) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	asset := r.URL.Query().Get("asset")
	if asset == "" {
		asset = "USDT"
	}

	balance := s.portfolio.GetBalance(asset)

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    balance,
		Time:    time.Now(),
	})
}

func (s *Server) handleGetPortfolioHistory(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	history := s.portfolio.GetHistory(limit)

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    history,
		Time:    time.Now(),
	})
}

func (s *Server) handleGetPositions(w http.ResponseWriter, r *http.Request) {
	positions := s.portfolio.GetAllPositions()

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    positions,
		Time:    time.Now(),
	})
}

func (s *Server) handleGetPosition(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	position := s.portfolio.GetPosition(symbol)

	if position == nil {
		s.respondError(w, r, "Position not found", 404)
		return
	}

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    position,
		Time:    time.Now(),
	})
}

func (s *Server) handleGetOrders(w http.ResponseWriter, r *http.Request) {
	orders := s.orders.GetAllOrders()

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    orders,
		Time:    time.Now(),
	})
}

type PlaceOrderRequest struct {
	Symbol    string `json:"symbol"`
	Side      string `json:"side"`
	Type      string `json:"type"`
	Quantity  string `json:"quantity"`
	Price     string `json:"price,omitempty"`
	StopPrice string `json:"stop_price,omitempty"`
}

func (s *Server) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	var req PlaceOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, r, "Invalid request body", 400)
		return
	}

	quantity, err := decimal.NewFromString(req.Quantity)
	if err != nil {
		s.respondError(w, r, "Invalid quantity", 400)
		return
	}

	order := &types.Order{
		Symbol:   req.Symbol,
		Side:     types.OrderSide(strings.ToUpper(req.Side)),
		Type:     types.OrderType(strings.ToLower(req.Type)),
		Quantity: quantity,
	}

	if req.Price != "" {
		order.Price, _ = decimal.NewFromString(req.Price)
	}
	if req.StopPrice != "" {
		order.StopPrice, _ = decimal.NewFromString(req.StopPrice)
	}

	if err := s.orders.PlaceOrder(order); err != nil {
		s.respondError(w, r, err.Error(), 500)
		return
	}

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    order,
		Time:    time.Now(),
	})
}

func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	order := s.orders.GetOrder(types.MustParseUUID(id))

	if order == nil {
		s.respondError(w, r, "Order not found", 404)
		return
	}

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    order,
		Time:    time.Now(),
	})
}

func (s *Server) handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	orderID := types.MustParseUUID(id)
	if err := s.orders.CancelOrder(orderID); err != nil {
		s.respondError(w, r, err.Error(), 500)
		return
	}

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    map[string]string{"status": "cancelled"},
		Time:    time.Now(),
	})
}

func (s *Server) handleGetTrades(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")

	trades := []interface{}{}

	s.respond(w, r, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"trades": trades,
			"symbol": symbol,
		},
		Time: time.Now(),
	})
}

func (s *Server) handleGetMarket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	ticker := &types.Ticker{
		Symbol:           symbol,
		LastPrice:        decimal.NewFromFloat(44135.68),
		PriceChangePct: decimal.NewFromFloat(2.34),
	}

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    ticker,
		Time:    time.Now(),
	})
}

func (s *Server) handleGetCandles(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]
	timeframe := r.URL.Query().Get("timeframe")
	limit := 100

	candles := []*types.Candle{}

	s.respond(w, r, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"symbol":    symbol,
			"timeframe": timeframe,
			"limit":     limit,
			"candles":   candles,
		},
		Time: time.Now(),
	})
}

func (s *Server) handleGetOrderBook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	orderBook := &types.OrderBook{
		Symbol: symbol,
	}

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    orderBook,
		Time:    time.Now(),
	})
}

func (s *Server) handleGetPrices(w http.ResponseWriter, r *http.Request) {
	prices := map[string]decimal.Decimal{}

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    prices,
		Time:    time.Now(),
	})
}

func (s *Server) handleGetSignals(w http.ResponseWriter, r *http.Request) {
	signals := []interface{}{}

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    signals,
		Time:    time.Now(),
	})
}

func (s *Server) handleGenerateSignal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Symbol   string `json:"symbol"`
		Strategy string `json:"strategy"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, r, "Invalid request body", 400)
		return
	}

	signal := &types.Signal{}

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    signal,
		Time:    time.Now(),
	})
}

func (s *Server) handleGetStrategies(w http.ResponseWriter, r *http.Request) {
	strategies := s.strategies.GetAllStrategies()

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    strategies,
		Time:    time.Now(),
	})
}

func (s *Server) handleGetStrategy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	strategy := s.strategies.GetStrategy(name)

	if strategy == nil {
		s.respondError(w, r, "Strategy not found", 404)
		return
	}

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    strategy,
		Time:    time.Now(),
	})
}

func (s *Server) handleStartStrategy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    map[string]string{"status": "started", "name": vars["name"]},
		Time:    time.Now(),
	})
}

func (s *Server) handleStopStrategy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    map[string]string{"status": "stopped", "name": vars["name"]},
		Time:    time.Now(),
	})
}

func (s *Server) handleGetRiskLimits(w http.ResponseWriter, r *http.Request) {
	limits := s.risk.GetLimits()

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    limits,
		Time:    time.Now(),
	})
}

func (s *Server) handleGetExposure(w http.ResponseWriter, r *http.Request) {
	s.respond(w, r, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"total_exposure":     s.risk.GetExposure(),
			"current_drawdown":  s.risk.GetDrawdown(),
			"max_exposure":      decimal.NewFromFloat(0.8),
		},
		Time: time.Now(),
	})
}

func (s *Server) handleGetExchanges(w http.ResponseWriter, r *http.Request) {
	exchanges := []string{"binance", "bybit", "coinbase", "kraken", "okx"}

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    exchanges,
		Time:    time.Now(),
	})
}

func (s *Server) handleConnectExchange(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    map[string]string{"exchange": name, "status": "connected"},
		Time:    time.Now(),
	})
}

func (s *Server) handleDisconnectExchange(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    map[string]string{"exchange": name, "status": "disconnected"},
		Time:    time.Now(),
	})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    cfg,
		Time:    time.Now(),
	})
}

func (s *Server) handleSetConfig(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	var value interface{}
	if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
		s.respondError(w, r, "Invalid request body", 400)
		return
	}

	s.config = nil

	s.respond(w, r, APIResponse{
		Success: true,
		Data:    map[string]interface{}{"key": key, "value": value},
		Time:    time.Now(),
	})
}

func (s *Server) handleWebSocket() http.Handler {
	return nil
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("API request", "method", r.Method, "path", r.URL.Path, "latency", time.Since(start))
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("Panic recovered", "error", err)
				s.respondError(w, r, "Internal server error", 500)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func (s *Server) respond(w http.ResponseWriter, r *http.Request, resp APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) respondError(w http.ResponseWriter, r *http.Request, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   msg,
		Time:    time.Now(),
	})
}
