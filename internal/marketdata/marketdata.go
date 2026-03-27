package marketdata

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
)

type Manager struct {
	config    Config
	ctx       context.Context
	cancel    context.CancelFunc
	feeds     map[string]*Feed
	storage   StorageAdapter
	mu        sync.RWMutex
	providers map[string]Provider
}

type Config struct {
	Enabled       bool
	RefreshRate   int
	BufferSize    int
	MaxCandles    int
	Providers     []string
}

type Feed struct {
	Symbol     string
	Exchange   types.Exchange
	Timeframe  types.Timeframe
	Candles    []*types.Candle
	handlers   []func(*types.Candle)
	ticker     *time.Ticker
	stopChan   chan struct{}
	mu         sync.RWMutex
}

type Provider interface {
	Name() string
	Connect(ctx context.Context) error
	Disconnect() error
	GetCandles(symbol string, timeframe types.Timeframe, limit int) ([]*types.Candle, error)
	GetTicker(symbol string) (*types.Ticker, error)
	GetOrderBook(symbol string, depth int) (*types.OrderBook, error)
	Subscribe(symbol string, channel string) error
	Unsubscribe(symbol string, channel string) error
}

type StorageAdapter interface {
	SaveCandle(candle *types.Candle) error
	GetCandles(symbol, timeframe string, start, end time.Time, limit int) ([]*types.Candle, error)
}

func NewManager(cfg Config) *Manager {
	if cfg.RefreshRate == 0 {
		cfg.RefreshRate = 1000
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 1000
	}
	if cfg.MaxCandles == 0 {
		cfg.MaxCandles = 10000
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config:    cfg,
		ctx:       ctx,
		cancel:    cancel,
		feeds:     make(map[string]*Feed),
		providers: make(map[string]Provider),
	}

	return m
}

func (m *Manager) Start(ctx context.Context) error {
	logger.Info("Starting market data manager")

	for name, provider := range m.providers {
		if err := provider.Connect(ctx); err != nil {
			logger.Error("Failed to connect provider", "provider", name, "error", err)
			continue
		}
		logger.Info("Connected to provider", "provider", name)
	}

	return nil
}

func (m *Manager) Stop() error {
	logger.Info("Stopping market data manager")

	m.cancel()

	for name, feed := range m.feeds {
		feed.Stop()
		delete(m.feeds, name)
	}

	for name, provider := range m.providers {
		if err := provider.Disconnect(); err != nil {
			logger.Error("Failed to disconnect provider", "provider", name, "error", err)
		}
	}

	return nil
}

func (m *Manager) AddProvider(name string, provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[name] = provider
}

func (m *Manager) Subscribe(symbol string, exchange types.Exchange, timeframe types.Timeframe, handler func(*types.Candle)) error {
	key := feedKey(symbol, exchange, timeframe)

	m.mu.Lock()
	defer m.mu.Unlock()

	feed, exists := m.feeds[key]
	if !exists {
		feed = &Feed{
			Symbol:    symbol,
			Exchange:  exchange,
			Timeframe: timeframe,
			Candles:   make([]*types.Candle, 0, m.config.MaxCandles),
			handlers:  make([]func(*types.Candle), 0),
			stopChan:  make(chan struct{}),
		}
		m.feeds[key] = feed
		go feed.run(m.ctx)
	}

	feed.handlers = append(feed.handlers, handler)

	logger.Info("Subscribed to feed",
		"symbol", symbol,
		"exchange", exchange,
		"timeframe", timeframe,
	)

	return nil
}

func (m *Manager) Unsubscribe(symbol string, exchange types.Exchange, timeframe types.Timeframe, handler func(*types.Candle)) error {
	key := feedKey(symbol, exchange, timeframe)

	m.mu.Lock()
	defer m.mu.Unlock()

	feed, exists := m.feeds[key]
	if !exists {
		return nil
	}

	for i, h := range feed.handlers {
		if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
			feed.handlers = append(feed.handlers[:i], feed.handlers[i+1:]...)
			break
		}
	}

	if len(feed.handlers) == 0 {
		feed.Stop()
		delete(m.feeds, key)
	}

	return nil
}

func (f *Feed) run(ctx context.Context) {
	f.ticker = time.NewTicker(f.Timeframe.Duration())
	defer f.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-f.stopChan:
			return
		case <-f.ticker.C:
			f.fetchCandle()
		}
	}
}

func (f *Feed) fetchCandle() {
	f.mu.Lock()
	defer f.mu.Unlock()

	candle := &types.Candle{
		Symbol:    f.Symbol,
		Exchange:  f.Exchange,
		Timeframe: string(f.Timeframe),
		Timestamp: time.Now(),
		Open:      types.PriceFromFloat(44135.68),
		High:      types.PriceFromFloat(44150.00),
		Low:       types.PriceFromFloat(44120.00),
		Close:     types.PriceFromFloat(44135.68),
		Volume:    types.PriceFromFloat(1234567),
		Closed:    true,
	}

	f.Candles = append(f.Candles, candle)
	if len(f.Candles) > 10000 {
		f.Candles = f.Candles[1:]
	}

	for _, handler := range f.handlers {
		go handler(candle)
	}
}

func (f *Feed) Stop() {
	select {
	case f.stopChan <- struct{}{}:
	default:
	}
	if f.ticker != nil {
		f.ticker.Stop()
	}
}

func (f *Feed) GetCandles(limit int) []*types.Candle {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if limit <= 0 || limit >= len(f.Candles) {
		return f.Candles
	}

	return f.Candles[len(f.Candles)-limit:]
}

func (m *Manager) GetCandles(symbol string, exchange types.Exchange, timeframe types.Timeframe, limit int) ([]*types.Candle, error) {
	m.mu.RLock()
	feed, exists := m.feeds[feedKey(symbol, exchange, timeframe)]
	m.mu.RUnlock()

	if exists {
		return feed.GetCandles(limit), nil
	}

	if m.storage != nil {
		return m.storage.GetCandles(symbol, string(timeframe), time.Time{}, time.Time{}, limit)
	}

	return nil, fmt.Errorf("no data available for %s", symbol)
}

func (m *Manager) GetTicker(symbol string, exchange types.Exchange) (*types.Ticker, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, provider := range m.providers {
		ticker, err := provider.GetTicker(symbol)
		if err == nil {
			return ticker, nil
		}
		logger.Warn("Provider failed to get ticker", "provider", name, "error", err)
	}

	return nil, fmt.Errorf("no provider available for ticker")
}

func (m *Manager) SetStorage(storage StorageAdapter) {
	m.storage = storage
}

func (m *Manager) GetFeed(symbol string, exchange types.Exchange, timeframe types.Timeframe) *Feed {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.feeds[feedKey(symbol, exchange, timeframe)]
}

func feedKey(symbol string, exchange types.Exchange, timeframe types.Timeframe) string {
	return fmt.Sprintf("%s:%s:%s", symbol, exchange, timeframe)
}

type Aggregator struct {
	symbol      string
	exchange    types.Exchange
	timeframes  []types.Timeframe
	aggregators map[types.Timeframe]*TimeframeAggregator
	mu          sync.RWMutex
}

type TimeframeAggregator struct {
	timeframe  types.Timeframe
	candles    map[int64]*types.Candle
	buffer     *types.Candle
	onComplete func(*types.Candle)
	mu         sync.RWMutex
}

func NewAggregator(symbol string, exchange types.Exchange, timeframes []types.Timeframe) *Aggregator {
	ag := &Aggregator{
		symbol:      symbol,
		exchange:    exchange,
		timeframes:  timeframes,
		aggregators: make(map[types.Timeframe]*TimeframeAggregator),
	}

	for _, tf := range timeframes {
		ag.aggregators[tf] = &TimeframeAggregator{
			timeframe: tf,
			candles:   make(map[int64]*types.Candle),
		}
	}

	return ag
}

func (a *Aggregator) OnCandle(candle *types.Candle) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for tf, agg := range a.aggregators {
		agg.addCandle(candle, tf.Duration())
	}
}

func (a *TimeframeAggregator) addCandle(candle *types.Candle, duration time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	bucket := candle.Timestamp.Truncate(duration).Unix()

	if existing, ok := a.candles[bucket]; ok {
		existing.High = types.MaxPrice(existing.High, candle.High)
		existing.Low = types.MinPrice(existing.Low, candle.Low)
		existing.Close = candle.Close
		existing.Volume = existing.Volume.Add(candle.Volume)
		existing.Trades++
		return
	}

	newCandle := &types.Candle{
		Symbol:    candle.Symbol,
		Exchange:  candle.Exchange,
		Timeframe: string(a.timeframe),
		Timestamp: candle.Timestamp.Truncate(duration),
		Open:      candle.Open,
		High:      candle.High,
		Low:       candle.Low,
		Close:     candle.Close,
		Volume:    candle.Volume,
		Trades:    1,
		Closed:    false,
	}

	a.candles[bucket] = newCandle
}

func (a *Aggregator) GetCandles(timeframe types.Timeframe, limit int) []*types.Candle {
	a.mu.RLock()
	defer a.mu.RUnlock()

	agg, ok := a.aggregators[timeframe]
	if !ok {
		return nil
	}

	agg.mu.RLock()
	defer agg.mu.RUnlock()

	candles := make([]*types.Candle, 0, len(agg.candles))
	for _, candle := range agg.candles {
		candles = append(candles, candle)
	}

	return candles
}

type Normalizer struct {
	mu       sync.RWMutex
	rules    map[string]*NormalizationRule
}

type NormalizationRule struct {
	Symbol      string
	Exchange    types.Exchange
	PriceScale  float64
	QtyScale    float64
	Delimiters  []string
}

func NewNormalizer() *Normalizer {
	return &Normalizer{
		rules: make(map[string]*NormalizationRule),
	}
}

func (n *Normalizer) AddRule(symbol string, exchange types.Exchange, rule *NormalizationRule) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.rules[fmt.Sprintf("%s:%s", symbol, exchange)] = rule
}

func (n *Normalizer) NormalizeSymbol(symbol string) (base, quote string, err error) {
	delimiters := []string{"/", "-", "_"}

	for _, delim := range delimiters {
		parts := splitSymbol(symbol, delim)
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}

	return "", "", fmt.Errorf("invalid symbol format: %s", symbol)
}

func splitSymbol(symbol, delim string) []string {
	result := make([]string, 0)
	current := ""

	for _, char := range symbol {
		if string(char) == delim {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"feeds":        len(m.feeds),
		"providers":    len(m.providers),
	}

	for key, feed := range m.feeds {
		feed.mu.RLock()
		stats["feed:"+key] = map[string]interface{}{
			"candles": len(feed.Candles),
			"handlers": len(feed.handlers),
		}
		feed.mu.RUnlock()
	}

	return stats
}
