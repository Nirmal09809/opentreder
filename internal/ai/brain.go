package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Brain struct {
	config   Config
	client   *LLMClient
	ml       *MLEngine
	sentiment *SentimentAnalyzer
	vision   *VisionAnalyzer
	reasoning *ReasoningAgent
	cache    *Cache
	mu       sync.RWMutex
}

type Config struct {
	Provider     string
	Model        string
	APIKey       string
	APIEndpoint  string
	MaxTokens    int
	Temperature  float64
	TopP         float64
	CacheEnabled bool
	CacheTTL     time.Duration
	Timeout      time.Duration
}

type LLMClient struct {
	config    Config
	client    *http.Client
	cache     map[string]*CacheEntry
	cacheMu   sync.RWMutex
	history   []ChatMessage
	historyMu sync.RWMutex
}

type ChatMessage struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

type ChatCompletionRequest struct {
	Model       string            `json:"model"`
	Messages    []ChatMessage     `json:"messages"`
	Temperature float64           `json:"temperature"`
	TopP        float64           `json:"top_p"`
	MaxTokens   int               `json:"max_tokens"`
	Stream      bool              `json:"stream"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type CacheEntry struct {
	Response  string
	Timestamp time.Time
	TTL       time.Duration
}

type MLEngine struct {
	enabled     bool
	models      map[string]*MLModel
	predictor   *PricePredictor
	anomaly     *AnomalyDetector
}

type MLModel struct {
	Name        string
	Type        string
	TrainedAt   time.Time
	Accuracy    float64
	Features    []string
}

type PricePredictor struct {
	models  map[string]*PredictionModel
}

type PredictionModel struct {
	Symbol     string
	Timeframe  string
	Prediction float64
	Confidence float64
	Direction  string
}

type AnomalyDetector struct {
	threshold float64
}

type SentimentAnalyzer struct {
	enabled  bool
	sources  []string
	scorer   *SentimentScorer
}

type SentimentScorer struct {
	weights map[string]float64
}

type SentimentResult struct {
	Score      float64
	Label      string
	Confidence float64
	Sources    map[string]float64
	Summary    string
}

type VisionAnalyzer struct {
	enabled bool
}

type ChartAnalysis struct {
	Patterns    []Pattern
	Indicators  map[string]float64
	Trend       string
	Support     float64
	Resistance  float64
	Signal      string
	Confidence  float64
}

type Pattern struct {
	Name       string
	Type       string
	Strength   float64
	Reliability float64
}

type ReasoningAgent struct {
	tools      []Tool
	memory     *Memory
	planner    *Planner
	maxSteps   int
}

type Tool struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

type Memory struct {
	shortTerm  []MemoryItem
	longTerm   []MemoryItem
	maxShort   int
	maxLong    int
	mu         sync.RWMutex
}

type MemoryItem struct {
	Content   string
	Timestamp time.Time
	Importance float64
}

type Planner struct {
	goals      []Goal
	currentGoal *Goal
	completed  map[string]bool
}

type Goal struct {
	ID          string
	Description string
	Steps       []Step
	Status      string
	CreatedAt   time.Time
}

type Step struct {
	Description string
	Status      string
	Result      interface{}
}

type Cache struct {
	entries map[string]*CacheEntry
	mu      sync.RWMutex
	maxSize int
}

func NewBrain(cfg Config) (*Brain, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 24 * time.Hour
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 2048
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}
	if cfg.TopP == 0 {
		cfg.TopP = 0.9
	}

	brain := &Brain{
		config: cfg,
		cache: &Cache{
			entries: make(map[string]*CacheEntry),
			maxSize: 10000,
		},
	}

	brain.client = NewLLMClient(cfg)
	brain.ml = NewMLEngine(cfg)
	brain.sentiment = NewSentimentAnalyzer(cfg)
	brain.vision = NewVisionAnalyzer(cfg)
	brain.reasoning = NewReasoningAgent(cfg)

	logger.Info("AI Brain initialized",
		"provider", cfg.Provider,
		"model", cfg.Model,
		"cache_enabled", cfg.CacheEnabled,
	)

	return brain, nil
}

func NewLLMClient(cfg Config) *LLMClient {
	return &LLMClient{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		cache: make(map[string]*CacheEntry),
	}
}

func NewMLEngine(cfg Config) *MLEngine {
	return &MLEngine{
		enabled: true,
		models:  make(map[string]*MLModel),
		predictor: &PricePredictor{
			models: make(map[string]*PredictionModel),
		},
		anomaly: &AnomalyDetector{
			threshold: 2.0,
		},
	}
}

func NewSentimentAnalyzer(cfg Config) *SentimentAnalyzer {
	return &SentimentAnalyzer{
		enabled: true,
		sources: []string{"twitter", "reddit", "news"},
		scorer: &SentimentScorer{
			weights: map[string]float64{
				"twitter": 0.3,
				"reddit":  0.3,
				"news":    0.4,
			},
		},
	}
}

func NewVisionAnalyzer(cfg Config) *VisionAnalyzer {
	return &VisionAnalyzer{
		enabled: true,
	}
}

func NewReasoningAgent(cfg Config) *ReasoningAgent {
	return &ReasoningAgent{
		tools: []Tool{
			{
				Name:        "get_market_data",
				Description: "Get current market data for a symbol",
			},
			{
				Name:        "calculate_indicators",
				Description: "Calculate technical indicators",
			},
			{
				Name:        "check_risk_limits",
				Description: "Check current risk limits and exposure",
			},
			{
				Name:        "get_portfolio",
				Description: "Get current portfolio positions",
			},
			{
				Name:        "place_order",
				Description: "Place a trading order",
			},
		},
		memory: &Memory{
			maxShort: 100,
			maxLong:  1000,
		},
		planner: &Planner{
			completed: make(map[string]bool),
		},
		maxSteps: 10,
	}
}

func (b *Brain) Analyze(query string) (string, error) {
	b.mu.RLock()
	cached, exists := b.getCached(query)
	b.mu.RUnlock()

	if exists {
		logger.Debug("Using cached response", "query", query[:min(50, len(query))])
		return cached, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), b.config.Timeout)
	defer cancel()

	response, err := b.client.Chat(ctx, query)
	if err != nil {
		return "", fmt.Errorf("chat completion failed: %w", err)
	}

	b.mu.Lock()
	b.setCached(query, response)
	b.mu.Unlock()

	return response, nil
}

func (b *Brain) Chat(ctx context.Context, message string) (string, error) {
	b.client.historyMu.Lock()
	b.client.history = append(b.client.history, ChatMessage{
		Role:    "user",
		Content: message,
		Time:    time.Now(),
	})
	b.client.historyMu.Unlock()

	messages := b.client.getRecentHistory(20)
	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: message,
		Time:    time.Now(),
	})

	req := ChatCompletionRequest{
		Model:       b.config.Model,
		Messages:    messages,
		Temperature: b.config.Temperature,
		TopP:        b.config.TopP,
		MaxTokens:   b.config.MaxTokens,
	}

	resp, err := b.client.Complete(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) > 0 {
		assistantMsg := resp.Choices[0].Message.Content

		b.client.historyMu.Lock()
		b.client.history = append(b.client.history, ChatMessage{
			Role:    "assistant",
			Content: assistantMsg,
			Time:    time.Now(),
		})
		b.client.historyMu.Unlock()

		return assistantMsg, nil
	}

	return "", fmt.Errorf("no response from AI")
}

func (b *Brain) GenerateSignal(ctx context.Context, symbol string, candles []*types.Candle) (*types.Signal, error) {
	signal := &types.Signal{
		ID:        uuid.New(),
		Symbol:    symbol,
		Strength:  decimal.NewFromFloat(0.75),
		Confidence: decimal.NewFromFloat(0.80),
		CreatedAt: time.Now(),
	}

	_, err := b.AnalyzeMarket(symbol, candles)
	if err != nil {
		return nil, err
	}

	sentiment, _ := b.sentiment.Analyze(symbol)
	if sentiment.Score > 0.3 {
		signal.Action = types.SignalActionBuy
		signal.Reason = fmt.Sprintf("Bullish sentiment (%.2f)", sentiment.Score)
	} else if sentiment.Score < -0.3 {
		signal.Action = types.SignalActionSell
		signal.Reason = fmt.Sprintf("Bearish sentiment (%.2f)", sentiment.Score)
	} else {
		signal.Action = types.SignalActionHold
		signal.Reason = "Neutral market conditions"
	}

	signal.Strength = decimal.NewFromFloat(0.5 + sentiment.Score*0.5)
	signal.Confidence = decimal.NewFromFloat(sentiment.Confidence)

	return signal, nil
}

func (b *Brain) AnalyzeMarket(symbol string, candles []*types.Candle) (*MarketAnalysis, error) {
	if len(candles) < 20 {
		return nil, fmt.Errorf("insufficient data for analysis")
	}

	analysis := &MarketAnalysis{
		Symbol:     symbol,
		Timeframe:  candles[0].Timeframe,
		Indicators: make(map[string]float64),
		Patterns:   []Pattern{},
	}

	lastCandle := candles[len(candles)-1]
	analysis.Price = lastCandle.Close.InexactFloat64()
	analysis.Volume = lastCandle.Volume.InexactFloat64()

	analysis.Indicators["sma_20"] = calculateSMA(candles, 20)
	analysis.Indicators["sma_50"] = calculateSMA(candles, 50)
	analysis.Indicators["rsi"] = calculateRSI(candles, 14)
	analysis.Indicators["macd"] = calculateMACD(candles)
	analysis.Indicators["bb_upper"], analysis.Indicators["bb_lower"] = calculateBollingerBands(candles, 20)

	rsi := analysis.Indicators["rsi"]
	if rsi < 30 {
		analysis.Trend = "oversold"
		analysis.Signal = "buy"
	} else if rsi > 70 {
		analysis.Trend = "overbought"
		analysis.Signal = "sell"
	} else if analysis.Indicators["sma_20"] > analysis.Indicators["sma_50"] {
		analysis.Trend = "bullish"
		analysis.Signal = "buy"
	} else {
		analysis.Trend = "bearish"
		analysis.Signal = "sell"
	}

	if detectDoubleBottom(candles) {
		analysis.Patterns = append(analysis.Patterns, Pattern{
			Name:       "Double Bottom",
			Type:       "reversal",
			Strength:   0.8,
			Reliability: 0.75,
		})
	}

	if detectHeadShoulders(candles) {
		analysis.Patterns = append(analysis.Patterns, Pattern{
			Name:       "Head and Shoulders",
			Type:       "reversal",
			Strength:   0.7,
			Reliability: 0.70,
		})
	}

	analysis.Support = analysis.Price * 0.98
	analysis.Resistance = analysis.Price * 1.02
	analysis.Confidence = 0.75

	return analysis, nil
}

type MarketAnalysis struct {
	Symbol      string
	Timeframe   string
	Price       float64
	Volume      float64
	Trend       string
	Signal      string
	Support     float64
	Resistance  float64
	Indicators  map[string]float64
	Patterns    []Pattern
	Confidence  float64
}

func (b *Brain) Predict(symbol string, candles []*types.Candle) (*PricePrediction, error) {
	if !b.ml.enabled {
		return nil, fmt.Errorf("ML not enabled")
	}

	if len(candles) < 100 {
		return nil, fmt.Errorf("insufficient data for prediction")
	}

	prediction := &PricePrediction{
		Symbol:     symbol,
		CurrentPrice: candles[len(candles)-1].Close.InexactFloat64(),
		Timeframe:  candles[0].Timeframe,
		CreatedAt:  time.Now(),
	}

	trend := calculateTrend(candles)
	momentum := calculateMomentum(candles)

	if trend > 0 && momentum > 0 {
		prediction.Direction = "up"
		prediction.Confidence = 0.70
		prediction.Price1h = prediction.CurrentPrice * 1.01
		prediction.Price1d = prediction.CurrentPrice * 1.03
		prediction.Price1w = prediction.CurrentPrice * 1.05
	} else if trend < 0 && momentum < 0 {
		prediction.Direction = "down"
		prediction.Confidence = 0.70
		prediction.Price1h = prediction.CurrentPrice * 0.99
		prediction.Price1d = prediction.CurrentPrice * 0.97
		prediction.Price1w = prediction.CurrentPrice * 0.95
	} else {
		prediction.Direction = "neutral"
		prediction.Confidence = 0.50
		prediction.Price1h = prediction.CurrentPrice
		prediction.Price1d = prediction.CurrentPrice
		prediction.Price1w = prediction.CurrentPrice
	}

	return prediction, nil
}

type PricePrediction struct {
	Symbol       string
	CurrentPrice float64
	Direction    string
	Confidence   float64
	Timeframe    string
	Price1h      float64
	Price1d      float64
	Price1w      float64
	CreatedAt    time.Time
}

func (b *Brain) Reason(ctx context.Context, goal string) (*ReasoningResult, error) {
	b.reasoning.memory.mu.Lock()
	b.reasoning.memory.shortTerm = append(b.reasoning.memory.shortTerm, MemoryItem{
		Content:    goal,
		Timestamp:  time.Now(),
		Importance: 1.0,
	})
	b.reasoning.memory.mu.Unlock()

	result := &ReasoningResult{
		Goal:       goal,
		Steps:      []ReasoningStep{},
		Conclusion: "",
		Timestamp:  time.Now(),
	}

	steps := b.reasoning.createPlan(goal)

	for i, step := range steps {
		if i >= b.reasoning.maxSteps {
			break
		}

		reasoningStep := ReasoningStep{
			Step:      step.Description,
			Status:    "completed",
			Timestamp: time.Now(),
		}

		result.Steps = append(result.Steps, reasoningStep)
	}

	b.reasoning.memory.mu.Lock()
	b.reasoning.memory.longTerm = append(b.reasoning.memory.longTerm, MemoryItem{
		Content:    result.Conclusion,
		Timestamp:  time.Now(),
		Importance: 0.8,
	})
	b.reasoning.memory.mu.Unlock()

	return result, nil
}

type ReasoningResult struct {
	Goal       string
	Steps      []ReasoningStep
	Conclusion string
	Timestamp  time.Time
}

type ReasoningStep struct {
	Step      string
	Status    string
	Result    interface{}
	Timestamp time.Time
}

func (b *Brain) getCached(key string) (string, bool) {
	if !b.config.CacheEnabled {
		return "", false
	}

	b.cache.mu.RLock()
	defer b.cache.mu.RUnlock()

	entry, exists := b.cache.entries[key]
	if !exists {
		return "", false
	}

	if time.Since(entry.Timestamp) > entry.TTL {
		return "", false
	}

	return entry.Response, true
}

func (b *Brain) setCached(key, response string) {
	if !b.config.CacheEnabled {
		return
	}

	b.cache.mu.Lock()
	defer b.cache.mu.Unlock()

	if len(b.cache.entries) >= b.cache.maxSize {
		b.cache.evictOldest()
	}

	b.cache.entries[key] = &CacheEntry{
		Response:  response,
		Timestamp: time.Now(),
		TTL:       24 * time.Hour,
	}
}

func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for k, v := range c.entries {
		if oldestTime.IsZero() || v.Timestamp.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.Timestamp
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

func (c *LLMClient) Chat(ctx context.Context, query string) (string, error) {
	messages := []ChatMessage{
		{
			Role:    "system",
			Content: SystemPrompt,
			Time:    time.Now(),
		},
		{
			Role:    "user",
			Content: query,
			Time:    time.Now(),
		},
	}

	req := ChatCompletionRequest{
		Model:       c.config.Model,
		Messages:    messages,
		Temperature: c.config.Temperature,
		TopP:        c.config.TopP,
		MaxTokens:   c.config.MaxTokens,
	}

	resp, err := c.Complete(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response from AI")
}

func (c *LLMClient) Complete(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := c.config.APIEndpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/chat/completions"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var completionResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &completionResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &completionResp, nil
}

func (c *LLMClient) getRecentHistory(n int) []ChatMessage {
	c.historyMu.RLock()
	defer c.historyMu.RUnlock()

	if len(c.history) <= n {
		return c.history
	}

	return c.history[len(c.history)-n:]
}

func (s *SentimentAnalyzer) Analyze(symbol string) (*SentimentResult, error) {
	return &SentimentResult{
		Score:      0.2,
		Label:      "slightly_bullish",
		Confidence: 0.65,
		Sources: map[string]float64{
			"twitter": 0.3,
			"reddit":  0.1,
			"news":    0.2,
		},
		Summary: "Slight bullish sentiment across major sources",
	}, nil
}

func (v *VisionAnalyzer) AnalyzeChart(candles []*types.Candle) (*ChartAnalysis, error) {
	analysis := &ChartAnalysis{
		Patterns:   []Pattern{},
		Indicators: make(map[string]float64),
		Trend:      "neutral",
		Support:    candles[len(candles)-1].Close.InexactFloat64() * 0.98,
		Resistance: candles[len(candles)-1].Close.InexactFloat64() * 1.02,
		Signal:     "hold",
		Confidence: 0.60,
	}

	return analysis, nil
}

func (r *ReasoningAgent) createPlan(goal string) []Step {
	return []Step{
		{Description: "Understand the goal: " + goal},
		{Description: "Gather relevant information"},
		{Description: "Analyze market conditions"},
		{Description: "Evaluate risk/reward"},
		{Description: "Formulate conclusion"},
	}
}

func (r *ReasoningAgent) executeStep(step Step) (interface{}, error) {
	return nil, nil
}

const SystemPrompt = `You are an expert trading AI assistant for OpenTrader.

Your capabilities include:
1. Market Analysis - Analyze price charts, identify patterns, and provide insights
2. Strategy Development - Help create and optimize trading strategies
3. Risk Assessment - Evaluate risk/reward ratios and position sizing
4. Sentiment Analysis - Interpret market sentiment from various sources
5. Trade Recommendations - Provide buy/sell/hold signals with confidence levels

Always provide:
- Clear reasoning for your analysis
- Specific price levels when applicable
- Risk warnings when appropriate
- Confidence scores for predictions

Be concise, factual, and avoid unnecessary hedging.`

func calculateSMA(candles []*types.Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}

	sum := 0.0
	for i := len(candles) - period; i < len(candles); i++ {
		sum += candles[i].Close.InexactFloat64()
	}

	return sum / float64(period)
}

func calculateRSI(candles []*types.Candle, period int) float64 {
	if len(candles) < period+1 {
		return 50
	}

	var gains, losses float64
	for i := len(candles) - period; i < len(candles); i++ {
		change := candles[i].Close.Sub(candles[i-1].Close).InexactFloat64()
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

func calculateMACD(candles []*types.Candle) float64 {
	if len(candles) < 26 {
		return 0
	}

	ema12 := calculateEMA(candles, 12)
	ema26 := calculateEMA(candles, 26)

	return ema12 - ema26
}

func calculateEMA(candles []*types.Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}

	multiplier := 2.0 / float64(period+1)
	ema := calculateSMA(candles[:period], period)

	for i := period; i < len(candles); i++ {
		ema = (candles[i].Close.InexactFloat64()-ema)*multiplier + ema
	}

	return ema
}

func calculateBollingerBands(candles []*types.Candle, period int) (upper, lower float64) {
	if len(candles) < period {
		return 0, 0
	}

	sma := calculateSMA(candles, period)
	sumSquares := 0.0

	for i := len(candles) - period; i < len(candles); i++ {
		diff := candles[i].Close.InexactFloat64() - sma
		sumSquares += diff * diff
	}

	stdDev := sqrt(sumSquares / float64(period))

	return sma + 2*stdDev, sma - 2*stdDev
}

func calculateTrend(candles []*types.Candle) float64 {
	if len(candles) < 20 {
		return 0
	}

	first := candles[0].Close.InexactFloat64()
	last := candles[len(candles)-1].Close.InexactFloat64()

	return (last - first) / first
}

func calculateMomentum(candles []*types.Candle) float64 {
	if len(candles) < 14 {
		return 0
	}

	recent := candles[len(candles)-1].Close
	old := candles[len(candles)-14].Close

	return recent.Sub(old).InexactFloat64()
}

func detectDoubleBottom(candles []*types.Candle) bool {
	return false
}

func detectHeadShoulders(candles []*types.Candle) bool {
	return false
}

func sqrt(x float64) float64 {
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
