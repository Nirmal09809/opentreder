package strategies

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opentreder/opentreder/pkg/events"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
)

type HotReloadConfig struct {
	WatchDir         string
	WatchInterval    time.Duration
	EnableAutoReload bool
	BackupDir        string
	MaxBackups       int
	OnReloadStart    func(name string)
	OnReloadComplete func(name string, success bool)
	OnReloadError    func(name string, err error)
}

type HotReload struct {
	config     HotReloadConfig
	manager    *StrategyManager
	eventBus   *events.EventBus
	watcher    *fsWatcher
	state      map[string]*StrategyStateSnapshot
	checksums  map[string]string
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	reloadCh   chan string
	errorLog   []ReloadError
	maxErrors  int
}

type StrategyStateSnapshot struct {
	ID           string
	Name         string
	Type         string
	State        StrategyState
	Positions    map[string]*types.Position
	Signals      []*types.Signal
	Candles      map[string][]*types.Candle
	Config       StrategyConfig
	Metrics      StrategyMetrics
	SnapshotTime time.Time
}

type ReloadError struct {
	StrategyName string
	Error        error
	Timestamp    time.Time
}

type ReloadEvent struct {
	StrategyName   string
	EventType      ReloadEventType
	OldChecksum    string
	NewChecksum    string
	Success        bool
	ErrorMessage   string
	Duration       time.Duration
	Timestamp      time.Time
}

type ReloadEventType string

const (
	ReloadEventStarted   ReloadEventType = "started"
	ReloadEventCompleted ReloadEventType = "completed"
	ReloadEventFailed    ReloadEventType = "failed"
	ReloadEventRolledBack ReloadEventType = "rolled_back"
)

type StrategyHotReload interface {
	Start() error
	Stop() error
	ReloadStrategy(name string) error
	ReloadAll() error
	EnableAutoReload(enabled bool)
	GetReloadHistory() []ReloadEvent
	GetStrategySnapshot(name string) (*StrategyStateSnapshot, error)
	Rollback(name string) error
}

type fsWatcher struct {
	watchDir   string
	interval   time.Duration
	fileEvents chan string
	done       chan struct{}
	mu         sync.Mutex
	running    bool
}

func NewHotReload(config HotReloadConfig, manager *StrategyManager, eventBus *events.EventBus) (*HotReload, error) {
	if config.WatchInterval == 0 {
		config.WatchInterval = 5 * time.Second
	}
	if config.MaxBackups == 0 {
		config.MaxBackups = 5
	}

	hr := &HotReload{
		config:    config,
		manager:   manager,
		eventBus:  eventBus,
		state:     make(map[string]*StrategyStateSnapshot),
		checksums: make(map[string]string),
		reloadCh:  make(chan string, 10),
		errorLog:  make([]ReloadError, 0),
		maxErrors: 100,
	}

	hr.ctx, hr.cancel = context.WithCancel(context.Background())

	if config.WatchDir != "" {
		hr.watcher = newFSWatcher(config.WatchDir, config.WatchInterval)
	}

	return hr, nil
}

func newFSWatcher(watchDir string, interval time.Duration) *fsWatcher {
	return &fsWatcher{
		watchDir:   watchDir,
		interval:   interval,
		fileEvents: make(chan string, 100),
		done:       make(chan struct{}),
	}
}

func (w *fsWatcher) Start(handler func(string)) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	go w.watchLoop(handler)
}

func (w *fsWatcher) watchLoop(handler func(string)) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	knownFiles := make(map[string]int64)

	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			w.scanDirectory(w.watchDir, knownFiles, handler)
		}
	}
}

func (w *fsWatcher) scanDirectory(dir string, knownFiles map[string]int64, handler func(string)) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			return nil
		}

		if modTime := info.ModTime().Unix(); knownFiles[path] != modTime {
			knownFiles[path] = modTime
			select {
			case w.fileEvents <- path:
				handler(path)
			default:
			}
		}

		return nil
	})
}

func (w *fsWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		close(w.done)
		w.running = false
	}
}

func (h *HotReload) Start() error {
	logger.Info("Starting strategy hot-reload system")

	if h.watcher != nil {
		h.watcher.Start(h.onFileChange)
	}

	go h.reloadWorker()

	return nil
}

func (h *HotReload) Stop() error {
	logger.Info("Stopping strategy hot-reload system")

	if h.watcher != nil {
		h.watcher.Stop()
	}

	h.cancel()
	close(h.reloadCh)

	return nil
}

func (h *HotReload) reloadWorker() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case name := <-h.reloadCh:
			if err := h.performReload(name); err != nil {
				logger.Error("Hot reload failed", "strategy", name, "error", err)
			}
		}
	}
}

func (h *HotReload) onFileChange(path string) {
	if !h.config.EnableAutoReload {
		return
	}

	filename := filepath.Base(path)
	name := filename[:len(filename)-len(filepath.Ext(filename))]

	logger.Info("File change detected", "path", path, "strategy", name)

	select {
	case h.reloadCh <- name:
	default:
		logger.Warn("Reload queue full, skipping", "strategy", name)
	}
}

func (h *HotReload) ReloadStrategy(name string) error {
	return h.performReload(name)
}

func (h *HotReload) ReloadAll() error {
	h.mu.RLock()
	strategies := h.manager.GetAllStrategies()
	h.mu.RUnlock()

	var lastErr error
	for name := range strategies {
		if err := h.performReload(name); err != nil {
			lastErr = err
			logger.Error("Failed to reload strategy", "name", name, "error", err)
		}
	}

	return lastErr
}

func (h *HotReload) performReload(name string) error {
	startTime := time.Now()

	if h.config.OnReloadStart != nil {
		h.config.OnReloadStart(name)
	}

	event := ReloadEvent{
		StrategyName: name,
		EventType:    ReloadEventStarted,
		Timestamp:    startTime,
	}

	if h.eventBus != nil {
		if ev, err := events.CreateEvent(events.EventTypeStrategyReloadStarted, uuid.New(), event); err == nil {
			h.eventBus.Publish(ev)
		}
	}

	h.mu.Lock()
	snapshot := h.captureSnapshot(name)
	oldChecksum := h.checksums[name]
	h.mu.Unlock()

	if snapshot == nil {
		return fmt.Errorf("strategy %s not found", name)
	}

	event.OldChecksum = oldChecksum

	newConfig, err := h.loadStrategyConfig(name)
	if err != nil {
		h.recordError(name, err)
		event.EventType = ReloadEventFailed
		event.ErrorMessage = err.Error()
		event.Duration = time.Since(startTime)

		if h.eventBus != nil {
			if ev, err := events.CreateEvent(events.EventTypeStrategyReloadFailed, uuid.New(), event); err == nil {
				h.eventBus.Publish(ev)
			}
		}
		return fmt.Errorf("failed to load new config: %w", err)
	}

	if err := h.backupCurrentState(name, snapshot); err != nil {
		logger.Warn("Failed to backup state", "strategy", name, "error", err)
	}

	newChecksum := h.calculateConfigChecksum(newConfig)

	if newChecksum == oldChecksum && oldChecksum != "" {
		logger.Debug("No changes detected", "strategy", name)
		return nil
	}

	event.NewChecksum = newChecksum

	if err := h.manager.RemoveStrategy(name); err != nil {
		h.recordError(name, err)
		event.EventType = ReloadEventFailed
		event.ErrorMessage = err.Error()
		event.Duration = time.Since(startTime)

		if h.eventBus != nil {
			if ev, err := events.CreateEvent(events.EventTypeStrategyReloadFailed, uuid.New(), event); err == nil {
				h.eventBus.Publish(ev)
			}
		}
		return fmt.Errorf("failed to stop strategy: %w", err)
	}

	newStrategy, err := h.createStrategyFromConfig(name, newConfig)
	if err != nil {
		h.rollback(name, snapshot)
		h.recordError(name, err)
		event.EventType = ReloadEventFailed
		event.ErrorMessage = err.Error()
		event.Duration = time.Since(startTime)

		if h.eventBus != nil {
			if ev, err := events.CreateEvent(events.EventTypeStrategyReloadFailed, uuid.New(), event); err == nil {
				h.eventBus.Publish(ev)
			}
		}
		return fmt.Errorf("failed to create new strategy: %w", err)
	}

	if err := h.manager.AddStrategy(name, newStrategy); err != nil {
		h.rollback(name, snapshot)
		h.recordError(name, err)
		event.EventType = ReloadEventFailed
		event.ErrorMessage = err.Error()
		event.Duration = time.Since(startTime)

		if h.eventBus != nil {
			if ev, err := events.CreateEvent(events.EventTypeStrategyReloadFailed, uuid.New(), event); err == nil {
				h.eventBus.Publish(ev)
			}
		}
		return fmt.Errorf("failed to add strategy: %w", err)
	}

	h.mu.Lock()
	h.checksums[name] = newChecksum
	h.state[name] = snapshot
	h.mu.Unlock()

	if err := newStrategy.Start(); err != nil {
		logger.Warn("Strategy started with warning", "name", name, "error", err)
	}

	event.EventType = ReloadEventCompleted
	event.Success = true
	event.Duration = time.Since(startTime)

	if h.eventBus != nil {
		if ev, err := events.CreateEvent(events.EventTypeStrategyReloadCompleted, uuid.New(), event); err == nil {
			h.eventBus.Publish(ev)
		}
	}

	if h.config.OnReloadComplete != nil {
		h.config.OnReloadComplete(name, true)
	}

	logger.Info("Strategy hot-reload completed",
		"strategy", name,
		"duration", event.Duration,
		"old_checksum", truncateChecksum(oldChecksum),
		"new_checksum", truncateChecksum(newChecksum))

	return nil
}

func (h *HotReload) captureSnapshot(name string) *StrategyStateSnapshot {
	strategy := h.manager.GetStrategy(name)
	if strategy == nil {
		return nil
	}

	state := strategy.GetState()
	metrics := strategy.GetMetrics()
	signals := strategy.GetSignals()

	positions := make(map[string]*types.Position)
	if bs, ok := strategy.(*BaseStrategy); ok {
		bs.mu.RLock()
		for k, v := range bs.positions {
			positions[k] = v
		}
		bs.mu.RUnlock()
	}

	snapshot := &StrategyStateSnapshot{
		ID:           strategy.GetID(),
		Name:         strategy.GetName(),
		Type:         getStrategyType(strategy),
		State:        state,
		Positions:    positions,
		Signals:      signals,
		Config:       h.getStrategyConfig(strategy),
		Metrics:      metrics,
		SnapshotTime: time.Now(),
	}

	return snapshot
}

func getStrategyType(strategy Strategy) string {
	switch strategy.(type) {
	case *GridStrategy:
		return "grid"
	case *DCAStrategy:
		return "dca"
	case *TrendStrategy:
		return "trend"
	case *ScalperStrategy:
		return "scalping"
	case *MarketMakingStrategy:
		return "market_making"
	case *MeanReversionStrategy:
		return "mean_reversion"
	case *BreakoutStrategy:
		return "breakout"
	case *MomentumStrategy:
		return "momentum"
	case *PairsTradingStrategy:
		return "pairs"
	case *ArbitrageStrategy:
		return "arbitrage"
	default:
		return "unknown"
	}
}

func (h *HotReload) getStrategyConfig(strategy Strategy) StrategyConfig {
	if bs, ok := strategy.(*BaseStrategy); ok {
		bs.mu.RLock()
		defer bs.mu.RUnlock()
		return bs.Config
	}
	return StrategyConfig{}
}

func (h *HotReload) loadStrategyConfig(name string) (StrategyConfig, error) {
	if h.config.WatchDir == "" {
		return StrategyConfig{}, fmt.Errorf("watch directory not configured")
	}

	patterns := []string{
		filepath.Join(h.config.WatchDir, name+".json"),
		filepath.Join(h.config.WatchDir, name+".yaml"),
		filepath.Join(h.config.WatchDir, name+".yml"),
	}

	var configData []byte
	var err error

	for _, pattern := range patterns {
		configData, err = os.ReadFile(pattern)
		if err == nil {
			break
		}
	}

	if configData == nil {
		return StrategyConfig{}, fmt.Errorf("config file not found for %s", name)
	}

	var config StrategyConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return StrategyConfig{}, fmt.Errorf("failed to parse config: %w", err)
	}

	return config, nil
}

func (h *HotReload) calculateConfigChecksum(config StrategyConfig) string {
	data, _ := json.Marshal(config)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (h *HotReload) backupCurrentState(name string, snapshot *StrategyStateSnapshot) error {
	if h.config.BackupDir == "" {
		return nil
	}

	if err := os.MkdirAll(h.config.BackupDir, 0755); err != nil {
		return err
	}

	backupFile := filepath.Join(h.config.BackupDir, fmt.Sprintf("%s_%d.backup", name, time.Now().Unix()))

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(backupFile, data, 0644); err != nil {
		return err
	}

	h.cleanOldBackups(name)

	return nil
}

func (h *HotReload) cleanOldBackups(name string) {
	if h.config.BackupDir == "" || h.config.MaxBackups <= 0 {
		return
	}

	pattern := filepath.Join(h.config.BackupDir, name+"_*.backup")
	matches, _ := filepath.Glob(pattern)

	if len(matches) <= h.config.MaxBackups {
		return
	}

	var oldest []string
	for _, match := range matches {
		if _, err := os.Stat(match); err != nil {
			continue
		}
		oldest = append(oldest, match)
	}

	for i := 0; i < len(oldest)-h.config.MaxBackups; i++ {
		os.Remove(oldest[i])
	}
}

func (h *HotReload) createStrategyFromConfig(name string, config StrategyConfig) (Strategy, error) {
	factory := NewStrategyFactory()

	params := make(map[string]interface{})
	for k, v := range config.Parameters {
		params[k] = v
	}

	strategy, err := factory.Create(config.Parameters["type"].(string), config.Symbol, params)
	if err != nil {
		return nil, err
	}

	if bs, ok := strategy.(*BaseStrategy); ok {
		bs.Config = config
	}

	return strategy, nil
}

func (h *HotReload) rollback(name string, snapshot *StrategyStateSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("no snapshot available for rollback")
	}

	logger.Info("Rolling back strategy", "name", name)

	strategy, err := h.createStrategyFromConfig(name, snapshot.Config)
	if err != nil {
		return fmt.Errorf("failed to recreate strategy: %w", err)
	}

	if err := h.manager.AddStrategy(name, strategy); err != nil {
		return fmt.Errorf("failed to add strategy: %w", err)
	}

	if err := strategy.Start(); err != nil {
		logger.Warn("Strategy rollback started with warning", "name", name, "error", err)
	}

	return nil
}

func (h *HotReload) recordError(name string, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.errorLog = append(h.errorLog, ReloadError{
		StrategyName: name,
		Error:        err,
		Timestamp:    time.Now(),
	})

	if len(h.errorLog) > h.maxErrors {
		h.errorLog = h.errorLog[len(h.errorLog)-h.maxErrors:]
	}

	if h.config.OnReloadError != nil {
		h.config.OnReloadError(name, err)
	}

	if h.eventBus != nil {
		if ev, err := events.CreateEvent(events.EventTypeStrategyReloadFailed, uuid.New(), map[string]interface{}{
			"strategy": name,
			"error":    err.Error(),
		}); err == nil {
			h.eventBus.Publish(ev)
		}
	}
}

func (h *HotReload) EnableAutoReload(enabled bool) {
	h.config.EnableAutoReload = enabled
	logger.Info("Auto-reload toggled", "enabled", enabled)
}

func (h *HotReload) GetReloadHistory() []ReloadEvent {
	return nil
}

func (h *HotReload) GetStrategySnapshot(name string) (*StrategyStateSnapshot, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	snapshot, exists := h.state[name]
	if !exists {
		return nil, fmt.Errorf("no snapshot found for strategy %s", name)
	}

	result := *snapshot
	return &result, nil
}

func (h *HotReload) Rollback(name string) error {
	h.mu.Lock()
	snapshot, exists := h.state[name]
	h.mu.Unlock()

	if !exists {
		return fmt.Errorf("no snapshot available for strategy %s", name)
	}

	return h.rollback(name, snapshot)
}

func truncateChecksum(checksum string) string {
	if len(checksum) < 8 {
		return checksum
	}
	return checksum[:8]
}
