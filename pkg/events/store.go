package events

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
)

type EventType string

const (
	EventTypeOrderSubmitted            EventType = "order_submitted"
	EventTypeOrderFilled              EventType = "order_filled"
	EventTypeOrderCancelled           EventType = "order_cancelled"
	EventTypeOrderRejected            EventType = "order_rejected"
	EventTypePositionOpened           EventType = "position_opened"
	EventTypePositionClosed          EventType = "position_closed"
	EventTypePositionUpdated         EventType = "position_updated"
	EventTypeTradeExecuted           EventType = "trade_executed"
	EventTypeBalanceUpdated          EventType = "balance_updated"
	EventTypeRiskAlert               EventType = "risk_alert"
	EventTypeStrategyStarted         EventType = "strategy_started"
	EventTypeStrategyStopped         EventType = "strategy_stopped"
	EventTypeSignalGenerated         EventType = "signal_generated"
	EventTypeMarketData              EventType = "market_data"
	EventTypeStrategyReloadStarted   EventType = "strategy_reload_started"
	EventTypeStrategyReloadCompleted EventType = "strategy_reload_completed"
	EventTypeStrategyReloadFailed    EventType = "strategy_reload_failed"
)

type Event struct {
	ID          uuid.UUID       `json:"id"`
	Type        EventType       `json:"type"`
	Timestamp   time.Time      `json:"timestamp"`
	Sequence    uint64         `json:"sequence"`
	Version     uint32         `json:"version"`
	AggregateID uuid.UUID       `json:"aggregate_id"`
	Data        json.RawMessage `json:"data"`
	Metadata    EventMetadata   `json:"metadata"`
}

type EventMetadata struct {
	CorrelationID uuid.UUID      `json:"correlation_id,omitempty"`
	CausationID   uuid.UUID      `json:"causation_id,omitempty"`
	UserID        string         `json:"user_id,omitempty"`
	IPAddress     string         `json:"ip_address,omitempty"`
	SessionID     string         `json:"session_id,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

type OrderSubmittedData struct {
	OrderID    string          `json:"order_id"`
	Symbol     string          `json:"symbol"`
	Exchange   types.Exchange  `json:"exchange"`
	Side       types.OrderSide `json:"side"`
	Type       types.OrderType `json:"type"`
	Quantity   string          `json:"quantity"`
	Price      string          `json:"price,omitempty"`
	StopPrice  string          `json:"stop_price,omitempty"`
	StrategyID string          `json:"strategy_id,omitempty"`
}

type OrderFilledData struct {
	OrderID        string `json:"order_id"`
	FilledQuantity string `json:"filled_quantity"`
	AvgFillPrice   string `json:"avg_fill_price"`
	Commission     string `json:"commission,omitempty"`
}

type OrderCancelledData struct {
	OrderID     string `json:"order_id"`
	Reason      string `json:"reason,omitempty"`
	CancelledBy string `json:"cancelled_by,omitempty"`
}

type OrderRejectedData struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
	Code    string `json:"code,omitempty"`
}

type PositionOpenedData struct {
	PositionID    string          `json:"position_id"`
	Symbol        string          `json:"symbol"`
	Exchange      types.Exchange  `json:"exchange"`
	Side          types.PositionSide `json:"side"`
	Quantity      string          `json:"quantity"`
	EntryPrice    string          `json:"entry_price"`
	MarginUsed    string          `json:"margin_used,omitempty"`
	StrategyID    string          `json:"strategy_id,omitempty"`
}

type PositionClosedData struct {
	PositionID     string `json:"position_id"`
	RealizedPnL    string `json:"realized_pnl"`
	ClosePrice     string `json:"close_price"`
	Duration       string `json:"duration"`
	ExitReason     string `json:"exit_reason,omitempty"`
}

type PositionUpdatedData struct {
	PositionID    string `json:"position_id"`
	UnrealizedPnL string `json:"unrealized_pnl"`
	MarketValue   string `json:"market_value"`
	LiquidationPrice string `json:"liquidation_price,omitempty"`
	MaintenanceMargin string `json:"maintenance_margin,omitempty"`
}

type TradeExecutedData struct {
	TradeID    string `json:"trade_id"`
	OrderID    string `json:"order_id"`
	PositionID string `json:"position_id,omitempty"`
	Symbol     string `json:"symbol"`
	Side       string `json:"side"`
	Price      string `json:"price"`
	Quantity   string `json:"quantity"`
	Commission string `json:"commission,omitempty"`
	Liquidity  string `json:"liquidity,omitempty"`
}

type BalanceUpdatedData struct {
	Asset         string `json:"asset"`
	Free          string `json:"free"`
	Locked        string `json:"locked"`
	Total         string `json:"total"`
	PreviousFree string `json:"previous_free"`
	PreviousLocked string `json:"previous_locked"`
	Change        string `json:"change"`
	Reason        string `json:"reason,omitempty"`
}

type RiskAlertData struct {
	AlertType    string `json:"alert_type"`
	Severity     string `json:"severity"`
	Message      string `json:"message"`
	PositionID   string `json:"position_id,omitempty"`
	VaR          string `json:"var,omitempty"`
	Drawdown     string `json:"drawdown,omitempty"`
	Exposure     string `json:"exposure,omitempty"`
	ActionTaken  string `json:"action_taken,omitempty"`
}

type SignalGeneratedData struct {
	SignalID    string `json:"signal_id"`
	StrategyID  string `json:"strategy_id"`
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	Action      string `json:"action"`
	Confidence  string `json:"confidence"`
	EntryPrice  string `json:"entry_price,omitempty"`
	StopLoss    string `json:"stop_loss,omitempty"`
	TakeProfit  string `json:"take_profit,omitempty"`
	Indicators  map[string]string `json:"indicators,omitempty"`
}

type MarketDataEvent struct {
	Symbol      string          `json:"symbol"`
	Exchange    types.Exchange  `json:"exchange"`
	DataType    string          `json:"data_type"`
	BidPrice    string          `json:"bid_price,omitempty"`
	AskPrice    string          `json:"ask_price,omitempty"`
	LastPrice   string          `json:"last_price,omitempty"`
	Volume      string          `json:"volume,omitempty"`
}

type EventStore struct {
	mu         sync.RWMutex
	events     []*Event
	snapshot   *Snapshot
	cfg        StoreConfig
	file       *os.File
	encoder    *json.Encoder
	sequence   uint64
	index      map[uuid.UUID][]uint64
	aggregates map[uuid.UUID][]uint64
}

type StoreConfig struct {
	Directory       string        `json:"directory"`
	MaxFileSize     int64         `json:"max_file_size"`
	SnapshotEvery   int           `json:"snapshot_every"`
	CompressFiles   bool          `json:"compress_files"`
	RetainDays      int           `json:"retain_days"`
}

type Snapshot struct {
	Version     uint32          `json:"version"`
	Timestamp   time.Time       `json:"timestamp"`
	Sequence    uint64          `json:"sequence"`
	State       json.RawMessage `json:"state"`
	NextCommands []CommandLog    `json:"next_commands,omitempty"`
}

type CommandLog struct {
	ID          uuid.UUID       `json:"id"`
	CommandType string          `json:"command_type"`
	Payload     json.RawMessage `json:"payload"`
	Timestamp   time.Time      `json:"timestamp"`
	Success     bool           `json:"success"`
	Error       string          `json:"error,omitempty"`
}

type EventHandler func(*Event) error

type EventBus struct {
	mu       sync.RWMutex
	subs     map[EventType][]EventHandler
	handlers []EventHandler
	bus      chan *Event
	done     chan struct{}
}

func NewEventStore(cfg StoreConfig) (*EventStore, error) {
	if err := os.MkdirAll(cfg.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create events directory: %w", err)
	}

	file, err := os.OpenFile(
		filepath.Join(cfg.Directory, "events.jsonl"),
		os.O_CREATE|os.O_APPEND|os.O_RDWR,
		0644,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open events file: %w", err)
	}

	store := &EventStore{
		cfg:        cfg,
		file:       file,
		encoder:    json.NewEncoder(file),
		events:     make([]*Event, 0),
		index:      make(map[uuid.UUID][]uint64),
		aggregates: make(map[uuid.UUID][]uint64),
	}

	if err := store.load(); err != nil {
		logger.Warn("Failed to load events", "error", err)
	}

	return store, nil
}

func (s *EventStore) Append(event *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	event.Sequence = s.sequence + 1
	event.Version = 1

	if err := s.encoder.Encode(event); err != nil {
		return fmt.Errorf("failed to encode event: %w", err)
	}

	s.events = append(s.events, event)
	s.sequence = event.Sequence
	s.index[event.ID] = append(s.index[event.ID], event.Sequence)
	s.aggregates[event.AggregateID] = append(s.aggregates[event.AggregateID], event.Sequence)

	if s.cfg.SnapshotEvery > 0 && s.sequence%uint64(s.cfg.SnapshotEvery) == 0 {
		if err := s.saveSnapshot(); err != nil {
			logger.Error("Failed to save snapshot", "error", err)
		}
	}

	return nil
}

func (s *EventStore) AppendBatch(events []*Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, event := range events {
		event.Sequence = s.sequence + 1
		event.Version = 1

		if err := s.encoder.Encode(event); err != nil {
			return fmt.Errorf("failed to encode event %s: %w", event.ID, err)
		}

		s.events = append(s.events, event)
		s.sequence = event.Sequence
	}

	return nil
}

func (s *EventStore) Get(id uuid.UUID) (*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sequences, ok := s.index[id]
	if !ok || len(sequences) == 0 {
		return nil, fmt.Errorf("event not found: %s", id)
	}

	return s.events[sequences[0]-1], nil
}

func (s *EventStore) GetByAggregate(aggregateID uuid.UUID) ([]*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sequences, ok := s.aggregates[aggregateID]
	if !ok {
		return []*Event{}, nil
	}

	events := make([]*Event, len(sequences))
	for i, seq := range sequences {
		if seq > 0 && seq <= uint64(len(s.events)) {
			events[i] = s.events[seq-1]
		}
	}

	return events, nil
}

func (s *EventStore) GetByType(eventType EventType, limit, offset int) ([]*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var events []*Event
	start := offset
	if start < 0 {
		start = 0
	}
	end := start + limit
	if end > len(s.events) {
		end = len(s.events)
	}

	for i := start; i < end; i++ {
		if s.events[i].Type == eventType {
			events = append(events, s.events[i])
		}
	}

	return events, nil
}

func (s *EventStore) GetRange(startSeq, endSeq uint64) ([]*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if startSeq < 1 {
		startSeq = 1
	}
	if endSeq > s.sequence {
		endSeq = s.sequence
	}

	events := make([]*Event, endSeq-startSeq+1)
	for i := startSeq - 1; i < endSeq; i++ {
		events[i-startSeq+1] = s.events[i]
	}

	return events, nil
}

func (s *EventStore) Replay(aggregateID uuid.UUID, handler EventHandler) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sequences, ok := s.aggregates[aggregateID]
	if !ok {
		return nil
	}

	for _, seq := range sequences {
		if seq > 0 && seq <= uint64(len(s.events)) {
			if err := handler(s.events[seq-1]); err != nil {
				return fmt.Errorf("handler error at sequence %d: %w", seq, err)
			}
		}
	}

	return nil
}

func (s *EventStore) ReplayFrom(aggregateID uuid.UUID, fromSequence uint64, handler EventHandler) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sequences, ok := s.aggregates[aggregateID]
	if !ok {
		return nil
	}

	for _, seq := range sequences {
		if seq >= fromSequence && seq <= s.sequence {
			if err := handler(s.events[seq-1]); err != nil {
				return fmt.Errorf("handler error at sequence %d: %w", seq, err)
			}
		}
	}

	return nil
}

func (s *EventStore) Count() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sequence
}

func (s *EventStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.saveSnapshot(); err != nil {
		logger.Error("Failed to save final snapshot", "error", err)
	}

	return s.file.Close()
}

func (s *EventStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stat, err := s.file.Stat()
	if err != nil {
		return err
	}

	if stat.Size() == 0 {
		return nil
	}

	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	scanner := json.NewDecoder(s.file)
	var lastSeq uint64

	for scanner.More() {
		var event Event
		if err := scanner.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			logger.Warn("Failed to decode event", "error", err)
			continue
		}

		s.events = append(s.events, &event)
		s.sequence = event.Sequence
		lastSeq = event.Sequence
		s.index[event.ID] = append(s.index[event.ID], event.Sequence)
		s.aggregates[event.AggregateID] = append(s.aggregates[event.AggregateID], event.Sequence)
	}

	if _, err := s.file.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	s.sequence = lastSeq
	return nil
}

func (s *EventStore) saveSnapshot() error {
	snapshot := &Snapshot{
		Version:   1,
		Timestamp: time.Now(),
		Sequence: s.sequence,
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	snapshotFile := filepath.Join(s.cfg.Directory, fmt.Sprintf("snapshot_%d.json", s.sequence))
	return os.WriteFile(snapshotFile, data, 0644)
}

func (s *EventStore) LoadSnapshot(sequence uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snapshotFile := filepath.Join(s.cfg.Directory, fmt.Sprintf("snapshot_%d.json", sequence))
	data, err := os.ReadFile(snapshotFile)
	if err != nil {
		return err
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}

	s.sequence = snapshot.Sequence
	return nil
}

func NewEventBus(bufferSize int) *EventBus {
	return &EventBus{
		subs:    make(map[EventType][]EventHandler),
		handlers: make([]EventHandler, 0),
		bus:    make(chan *Event, bufferSize),
		done:   make(chan struct{}),
	}
}

func (b *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[eventType] = append(b.subs[eventType], handler)
}

func (b *EventBus) SubscribeAll(handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, handler)
}

func (b *EventBus) Publish(event *Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, handler := range b.subs[event.Type] {
		if err := handler(event); err != nil {
			logger.Error("Event handler error", "type", event.Type, "error", err)
		}
	}

	for _, handler := range b.handlers {
		if err := handler(event); err != nil {
			logger.Error("Global event handler error", "type", event.Type, "error", err)
		}
	}
}

func (b *EventBus) Start() {
	go func() {
		for {
			select {
			case <-b.done:
				return
			case event := <-b.bus:
				b.Publish(event)
			}
		}
	}()
}

func (b *EventBus) Stop() {
	close(b.done)
}

func CreateEvent(eventType EventType, aggregateID uuid.UUID, data interface{}) (*Event, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %w", err)
	}

	return &Event{
		ID:          uuid.New(),
		Type:        eventType,
		Timestamp:   time.Now(),
		AggregateID: aggregateID,
		Data:        dataBytes,
		Metadata:    EventMetadata{},
	}, nil
}

func CreateOrderEvent(order *types.Order, eventType EventType) (*Event, error) {
	var data interface{}

	switch eventType {
	case EventTypeOrderSubmitted:
		data = OrderSubmittedData{
			OrderID:    order.ID.String(),
			Symbol:     order.Symbol,
			Exchange:   order.Exchange,
			Side:       order.Side,
			Type:       order.Type,
			Quantity:   order.Quantity.String(),
			Price:      order.Price.String(),
			StopPrice:  order.StopPrice.String(),
		}
	case EventTypeOrderFilled:
		data = OrderFilledData{
			OrderID:        order.ID.String(),
			FilledQuantity: order.FilledQuantity.String(),
			AvgFillPrice:   order.AvgFillPrice.String(),
		}
	case EventTypeOrderCancelled:
		data = OrderCancelledData{
			OrderID: order.ID.String(),
		}
	case EventTypeOrderRejected:
		data = OrderRejectedData{
			OrderID: order.ID.String(),
			Reason:  "unknown",
		}
	}

	return CreateEvent(eventType, order.ID, data)
}

func CreatePositionEvent(pos *types.Position, eventType EventType) (*Event, error) {
	var data interface{}

	switch eventType {
	case EventTypePositionOpened:
		data = PositionOpenedData{
			PositionID: pos.ID.String(),
			Symbol:     pos.Symbol,
			Exchange:   pos.Exchange,
			Side:       pos.Side,
			Quantity:   pos.Quantity.String(),
			EntryPrice: pos.AvgEntryPrice.String(),
		}
	case EventTypePositionClosed:
		data = PositionClosedData{
			PositionID:  pos.ID.String(),
			RealizedPnL: pos.RealizedPnL.String(),
			ClosePrice:  pos.CurrentPrice.String(),
		}
	case EventTypePositionUpdated:
		data = PositionUpdatedData{
			PositionID:    pos.ID.String(),
			UnrealizedPnL: pos.UnrealizedPnL.String(),
		}
	}

	return CreateEvent(eventType, uuid.New(), data)
}
