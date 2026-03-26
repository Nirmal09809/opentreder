package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/opentreder/opentreder/pkg/logger"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	addr        string
	clients     map[*Client]bool
	rooms       map[string]map[*Client]bool
	broadcast   chan *Message
	register    chan *Client
	unregister  chan *Client
	handlers    map[string]MessageHandler
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

type Client struct {
	conn     *websocket.Conn
	server   *Server
	send     chan []byte
	rooms    map[string]bool
	mu       sync.RWMutex
	ID       string
	Metadata map[string]interface{}
}

type Message struct {
	Type      string          `json:"type"`
	Action    string          `json:"action"`
	Channel   string          `json:"channel,omitempty"`
	Symbol    string          `json:"symbol,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	ClientID  string          `json:"client_id,omitempty"`
}

type MessageHandler func(client *Client, msg *Message)

type Subscription struct {
	Channel string `json:"channel"`
	Symbol  string `json:"symbol,omitempty"`
}

type TickerUpdate struct {
	Symbol        string  `json:"symbol"`
	Exchange      string  `json:"exchange"`
	Price         float64 `json:"price"`
	Change24h     float64 `json:"change_24h"`
	Volume24h     float64 `json:"volume_24h"`
	High24h       float64 `json:"high_24h"`
	Low24h        float64 `json:"low_24h"`
	Timestamp     int64   `json:"timestamp"`
}

type OrderBookUpdate struct {
	Symbol    string          `json:"symbol"`
	Exchange  string          `json:"exchange"`
	Bids      [][]interface{} `json:"bids"`
	Asks      [][]interface{} `json:"asks"`
	Timestamp int64          `json:"timestamp"`
}

type TradeUpdate struct {
	ID        string  `json:"id"`
	Symbol    string  `json:"symbol"`
	Exchange  string  `json:"exchange"`
	Side      string  `json:"side"`
	Price     float64 `json:"price"`
	Quantity  float64 `json:"quantity"`
	Timestamp int64   `json:"timestamp"`
}

type CandleUpdate struct {
	Symbol    string  `json:"symbol"`
	Exchange  string  `json:"exchange"`
	Timeframe string `json:"timeframe"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	Timestamp int64   `json:"timestamp"`
}

type PositionUpdate struct {
	ID            string  `json:"id"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	Quantity      float64 `json:"quantity"`
	EntryPrice    float64 `json:"entry_price"`
	CurrentPrice  float64 `json:"current_price"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
	ROI           float64 `json:"roi"`
}

type OrderUpdate struct {
	ID       string `json:"id"`
	Symbol   string `json:"symbol"`
	Side     string `json:"side"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Filled   float64 `json:"filled"`
}

type PortfolioUpdate struct {
	TotalValue     float64 `json:"total_value"`
	CashBalance   float64 `json:"cash_balance"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
	DayPnL        float64 `json:"day_pnl"`
}

func NewServer(addr string) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		addr:       addr,
		clients:    make(map[*Client]bool),
		rooms:      make(map[string]map[*Client]bool),
		broadcast:  make(chan *Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		handlers:   make(map[string]MessageHandler),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleConnection)
	mux.HandleFunc("/ws", s.handleConnection)

	server := &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go s.run()
	go s.startPinger()

	logger.Info("WebSocket server starting", "addr", s.addr)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("WebSocket server error", "error", err)
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	s.cancel()

	s.mu.Lock()
	for client := range s.clients {
		client.conn.Close()
	}
	s.mu.Unlock()

	return nil
}

func (s *Server) run() {
	for {
		select {
		case <-s.ctx.Done():
			return

		case client := <-s.register:
			s.mu.Lock()
			s.clients[client] = true
			s.mu.Unlock()

			logger.Info("Client connected",
				"client_id", client.ID,
				"total_clients", len(s.clients),
			)

			s.sendToClient(client, &Message{
				Type:      "system",
				Action:    "connected",
				Timestamp: time.Now(),
			})

		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
			}
			s.mu.Unlock()

			logger.Info("Client disconnected",
				"client_id", client.ID,
				"total_clients", len(s.clients),
			)

		case message := <-s.broadcast:
			s.mu.RLock()
			for client := range s.clients {
				select {
				case client.send <- mustMarshal(message):
				default:
					close(client.send)
					delete(s.clients, client)
				}
			}
			s.mu.RUnlock()
		}
	}
}

func (s *Server) startPinger() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			for client := range s.clients {
				if err := client.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
					s.unregister <- client
				}
			}
			s.mu.RUnlock()
		}
	}
}

func (s *Server) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("WebSocket upgrade failed", "error", err)
		return
	}

	client := &Client{
		conn:    conn,
		server:  s,
		send:    make(chan []byte, 256),
		rooms:   make(map[string]bool),
		ID:      generateID(),
		Metadata: make(map[string]interface{}),
	}

	s.register <- client

	go client.writePump()
	go client.readPump()
}

func (s *Server) RegisterHandler(action string, handler MessageHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[action] = handler
}

func (s *Server) Broadcast(message *Message) {
	select {
	case s.broadcast <- message:
	default:
		logger.Warn("Broadcast channel full, dropping message")
	}
}

func (s *Server) BroadcastToRoom(room string, message *Message) {
	s.mu.RLock()
	clients, exists := s.rooms[room]
	s.mu.RUnlock()

	if !exists {
		return
	}

	data := mustMarshal(message)

	s.mu.RLock()
	for client := range clients {
		select {
		case client.send <- data:
		default:
		}
	}
	s.mu.RUnlock()
}

func (s *Server) SendToClient(clientID string, message *Message) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for client := range s.clients {
		if client.ID == clientID {
			select {
			case client.send <- mustMarshal(message):
			default:
			}
			return
		}
	}
}

func (s *Server) JoinRoom(client *Client, room string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.rooms[room]; !exists {
		s.rooms[room] = make(map[*Client]bool)
	}

	s.rooms[room][client] = true
	client.mu.Lock()
	client.rooms[room] = true
	client.mu.Unlock()

	logger.Debug("Client joined room", "client_id", client.ID, "room", room)
}

func (s *Server) LeaveRoom(client *Client, room string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if clients, exists := s.rooms[room]; exists {
		delete(clients, client)
		if len(clients) == 0 {
			delete(s.rooms, room)
		}
	}

	client.mu.Lock()
	delete(client.rooms, room)
	client.mu.Unlock()

	logger.Debug("Client left room", "client_id", client.ID, "room", room)
}

func (s *Server) GetClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

func (s *Server) GetRoomClients(room string) []*Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients, exists := s.rooms[room]
	if !exists {
		return nil
	}

	result := make([]*Client, 0, len(clients))
	for client := range clients {
		result = append(result, client)
	}

	return result
}

func (c *Client) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(65536)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("WebSocket read error", "error", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Error("Failed to parse message", "error", err)
			continue
		}

		msg.ClientID = c.ID
		msg.Timestamp = time.Now()

		c.handleMessage(&msg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg *Message) {
	switch msg.Type {
	case "subscribe":
		c.handleSubscribe(msg)

	case "unsubscribe":
		c.handleUnsubscribe(msg)

	case "ping":
		c.sendToClient(c, &Message{
			Type:      "pong",
			Timestamp: time.Now(),
		})

	default:
		c.server.mu.RLock()
		handler, exists := c.server.handlers[msg.Action]
		c.server.mu.RUnlock()

		if exists {
			handler(c, msg)
		}
	}
}

func (c *Client) handleSubscribe(msg *Message) {
	channel := msg.Channel
	if channel == "" {
		return
	}

	c.server.JoinRoom(c, channel)

	c.sendToClient(c, &Message{
		Type:      "subscribed",
		Channel:   channel,
		Symbol:    msg.Symbol,
		Timestamp: time.Now(),
	})

	logger.Info("Client subscribed",
		"client_id", c.ID,
		"channel", channel,
		"symbol", msg.Symbol,
	)
}

func (c *Client) handleUnsubscribe(msg *Message) {
	channel := msg.Channel
	if channel == "" {
		return
	}

	c.server.LeaveRoom(c, channel)

	c.sendToClient(c, &Message{
		Type:      "unsubscribed",
		Channel:   channel,
		Timestamp: time.Now(),
	})

	logger.Info("Client unsubscribed",
		"client_id", c.ID,
		"channel", channel,
	)
}

func (c *Client) sendToClient(client *Client, msg *Message) {
	select {
	case client.send <- mustMarshal(msg):
	default:
	}
}

func (s *Server) EmitTicker(symbol string, ticker *TickerUpdate) {
	s.BroadcastToRoom("tickers:"+symbol, &Message{
		Type: "ticker",
		Data: mustMarshal(ticker),
	})
}

func (s *Server) EmitOrderBook(symbol string, orderbook *OrderBookUpdate) {
	s.BroadcastToRoom("orderbook:"+symbol, &Message{
		Type: "orderbook",
		Data: mustMarshal(orderbook),
	})
}

func (s *Server) EmitTrade(symbol string, trade *TradeUpdate) {
	s.BroadcastToRoom("trades:"+symbol, &Message{
		Type: "trade",
		Data: mustMarshal(trade),
	})
}

func (s *Server) EmitCandle(symbol, timeframe string, candle *CandleUpdate) {
	s.BroadcastToRoom("candles:"+symbol+":"+timeframe, &Message{
		Type: "candle",
		Data: mustMarshal(candle),
	})
}

func (s *Server) EmitPosition(position *PositionUpdate) {
	s.Broadcast(&Message{
		Type: "position",
		Data: mustMarshal(position),
	})
}

func (s *Server) EmitOrder(order *OrderUpdate) {
	s.Broadcast(&Message{
		Type: "order",
		Data: mustMarshal(order),
	})
}

func (s *Server) EmitPortfolio(portfolio *PortfolioUpdate) {
	s.Broadcast(&Message{
		Type: "portfolio",
		Data: mustMarshal(portfolio),
	})
}

func generateID() string {
	return fmt.Sprintf("client_%d", time.Now().UnixNano())
}

func mustMarshal(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}
