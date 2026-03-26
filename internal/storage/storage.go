package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opentreder/opentreder/pkg/config"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
	"github.com/shopspring/decimal"
)

type Manager struct {
	sqlite    *SQLiteDB
	postgres  *PostgresDB
	redis     *RedisCache
	mode      string
	mu        sync.RWMutex
}

type SQLiteDB struct {
	db     *sql.DB
	path   string
	mu     sync.RWMutex
}

type PostgresDB struct {
	db    *sql.DB
	host  string
	port  int
	mu    sync.RWMutex
}

type RedisCache struct {
	addr    string
	password string
	db      int
}

func NewManager() (*Manager, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, fmt.Errorf("config not loaded")
	}

	m := &Manager{
		mode: cfg.Database.Mode,
	}

	var err error

	if cfg.Database.SQLite.Enabled {
		m.sqlite, err = NewSQLiteDB(cfg.Database.SQLite.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize SQLite: %w", err)
		}
	}

	if cfg.Database.PostgreSQL.Enabled {
		m.postgres, err = NewPostgresDB(&cfg.Database.PostgreSQL)
		if err != nil {
			logger.Warn("PostgreSQL connection failed, continuing without it", "error", err)
		}
	}

	if cfg.Cache.Redis.Enabled {
		m.redis, err = NewRedisCache(&cfg.Cache.Redis)
		if err != nil {
			logger.Warn("Redis connection failed, continuing without it", "error", err)
		}
	}

	logger.Info("Storage manager initialized",
		"mode", m.mode,
		"sqlite", m.sqlite != nil,
		"postgres", m.postgres != nil,
		"redis", m.redis != nil,
	)

	return m, nil
}

func NewSQLiteDB(path string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping SQLite: %w", err)
	}

	sqlite := &SQLiteDB{
		db:   db,
		path: path,
	}

	if err := sqlite.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Info("SQLite database initialized", "path", path)

	return sqlite, nil
}

func (s *SQLiteDB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id TEXT PRIMARY KEY,
			client_order_id TEXT,
			exchange TEXT NOT NULL,
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			price TEXT,
			stop_price TEXT,
			quantity TEXT NOT NULL,
			filled_quantity TEXT DEFAULT '0',
			remaining_qty TEXT,
			avg_fill_price TEXT,
			commission TEXT DEFAULT '0',
			commission_asset TEXT,
			time_in_force TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			filled_at DATETIME,
			cancelled_at DATETIME,
			strategy_id TEXT,
			metadata TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_symbol ON orders(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_created ON orders(created_at)`,

		`CREATE TABLE IF NOT EXISTS positions (
			id TEXT PRIMARY KEY,
			exchange TEXT NOT NULL,
			symbol TEXT NOT NULL,
			asset_type TEXT NOT NULL,
			side TEXT NOT NULL,
			quantity TEXT NOT NULL,
			avg_entry_price TEXT NOT NULL,
			current_price TEXT,
			unrealized_pnl TEXT DEFAULT '0',
			realized_pnl TEXT DEFAULT '0',
			roi TEXT DEFAULT '0',
			leverage TEXT DEFAULT '1',
			isolated_margin TEXT,
			liquidation_price TEXT,
			opened_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			strategy_id TEXT,
			UNIQUE(exchange, symbol)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_positions_symbol ON positions(symbol)`,

		`CREATE TABLE IF NOT EXISTS trades (
			id TEXT PRIMARY KEY,
			exchange TEXT NOT NULL,
			symbol TEXT NOT NULL,
			side TEXT NOT NULL,
			price TEXT NOT NULL,
			quantity TEXT NOT NULL,
			quote_qty TEXT,
			commission TEXT DEFAULT '0',
			timestamp DATETIME NOT NULL,
			order_id TEXT,
			is_buyer_maker INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_symbol ON trades(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_timestamp ON trades(timestamp)`,

		`CREATE TABLE IF NOT EXISTS candles (
			symbol TEXT NOT NULL,
			exchange TEXT NOT NULL,
			timeframe TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			open TEXT NOT NULL,
			high TEXT NOT NULL,
			low TEXT NOT NULL,
			close TEXT NOT NULL,
			volume TEXT NOT NULL,
			quote_volume TEXT,
			taker_buy_volume TEXT,
			trades INTEGER DEFAULT 0,
			closed INTEGER DEFAULT 1,
			PRIMARY KEY (symbol, exchange, timeframe, timestamp)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_candles_timestamp ON candles(timestamp)`,

		`CREATE TABLE IF NOT EXISTS strategies (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			description TEXT,
			enabled INTEGER DEFAULT 0,
			mode TEXT DEFAULT 'paper',
			config TEXT,
			parameters TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS signals (
			id TEXT PRIMARY KEY,
			strategy_id TEXT,
			symbol TEXT NOT NULL,
			exchange TEXT NOT NULL,
			action TEXT NOT NULL,
			strength TEXT,
			price TEXT,
			stop_loss TEXT,
			take_profit TEXT,
			quantity TEXT,
			reason TEXT,
			confidence TEXT,
			created_at DATETIME NOT NULL,
			metadata TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_signals_symbol ON signals(symbol)`,
		`CREATE INDEX IF NOT EXISTS idx_signals_created ON signals(created_at)`,

		`CREATE TABLE IF NOT EXISTS balances (
			asset TEXT NOT NULL,
			exchange TEXT NOT NULL,
			free TEXT DEFAULT '0',
			locked TEXT DEFAULT '0',
			total TEXT DEFAULT '0',
			usd_value TEXT DEFAULT '0',
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (asset, exchange)
		)`,

		`CREATE TABLE IF NOT EXISTS portfolio_snapshots (
			id TEXT PRIMARY KEY,
			total_value TEXT NOT NULL,
			cash_balance TEXT NOT NULL,
			equity TEXT NOT NULL,
			margin_used TEXT,
			margin_available TEXT,
			unrealized_pnl TEXT DEFAULT '0',
			realized_pnl TEXT DEFAULT '0',
			day_pnl TEXT DEFAULT '0',
			snapshot_time DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_portfolio_time ON portfolio_snapshots(snapshot_time)`,
	}

	for i, migration := range migrations {
		if _, err := s.db.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	return nil
}

func NewPostgresDB(cfg *config.PostgreSQLConfig) (*PostgresDB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	logger.Info("PostgreSQL database initialized",
		"host", cfg.Host,
		"database", cfg.Database,
	)

	return &PostgresDB{
		db:   db,
		host: cfg.Host,
		port: cfg.Port,
	}, nil
}

func NewRedisCache(cfg *config.RedisConfig) (*RedisCache, error) {
	return &RedisCache{
		addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		password: cfg.Password,
		db:       cfg.Database,
	}, nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	if m.sqlite != nil {
		if err := m.sqlite.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if m.postgres != nil {
		if err := m.postgres.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing storage: %v", errs)
	}

	return nil
}

func (m *Manager) SaveOrder(order *types.Order) error {
	if m.sqlite == nil {
		return fmt.Errorf("SQLite not available")
	}

	m.sqlite.mu.Lock()
	defer m.sqlite.mu.Unlock()

	query := `INSERT OR REPLACE INTO orders 
		(id, client_order_id, exchange, symbol, side, type, status, price, stop_price,
		quantity, filled_quantity, remaining_qty, avg_fill_price, commission, commission_asset,
		time_in_force, created_at, updated_at, filled_at, cancelled_at, strategy_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := m.sqlite.db.Exec(query,
		order.ID.String(),
		order.ClientOrderID,
		string(order.Exchange),
		order.Symbol,
		string(order.Side),
		string(order.Type),
		string(order.Status),
		order.Price.String(),
		order.StopPrice.String(),
		order.Quantity.String(),
		order.FilledQuantity.String(),
		order.RemainingQty.String(),
		order.AvgFillPrice.String(),
		order.Commission.String(),
		order.CommissionAsset,
		string(order.TimeInForce),
		order.CreatedAt,
		order.UpdatedAt,
		order.FilledAt,
		order.CancelledAt,
		order.StrategyID.String(),
	)

	return err
}

func (m *Manager) GetOrders(symbol string, limit int) ([]*types.Order, error) {
	if m.sqlite == nil {
		return nil, fmt.Errorf("SQLite not available")
	}

	m.sqlite.mu.RLock()
	defer m.sqlite.mu.RUnlock()

	query := "SELECT * FROM orders"
	args := []interface{}{}

	if symbol != "" {
		query += " WHERE symbol = ?"
		args = append(args, symbol)
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := m.sqlite.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*types.Order
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	return orders, nil
}

func scanOrder(rows *sql.Rows) (*types.Order, error) {
	var order types.Order
	var id, clientOrderID, exchange, symbol, side, orderType, status string
	var price, stopPrice, quantity, filledQty, remainingQty, avgFillPrice, commission string
	var commissionAsset, timeInForce string
	var strategyID string
	var createdAt, updatedAt time.Time
	var filledAt, cancelledAt sql.NullTime

	err := rows.Scan(
		&id, &clientOrderID, &exchange, &symbol, &side, &orderType, &status,
		&price, &stopPrice, &quantity, &filledQty, &remainingQty, &avgFillPrice,
		&commission, &commissionAsset, &timeInForce, &createdAt, &updatedAt,
		&filledAt, &cancelledAt, &strategyID,
	)
	if err != nil {
		return nil, err
	}

	order.ID, _ = uuid.Parse(id)
	order.ClientOrderID = clientOrderID
	order.Exchange = types.Exchange(exchange)
	order.Symbol = symbol
	order.Side = types.OrderSide(side)
	order.Type = types.OrderType(orderType)
	order.Status = types.OrderStatus(status)
	order.Price, _ = decimal.NewFromString(price)
	order.StopPrice, _ = decimal.NewFromString(stopPrice)
	order.Quantity, _ = decimal.NewFromString(quantity)
	order.FilledQuantity, _ = decimal.NewFromString(filledQty)
	order.RemainingQty, _ = decimal.NewFromString(remainingQty)
	order.AvgFillPrice, _ = decimal.NewFromString(avgFillPrice)
	order.Commission, _ = decimal.NewFromString(commission)
	order.CommissionAsset = commissionAsset
	order.TimeInForce = types.TimeInForce(timeInForce)
	order.CreatedAt = createdAt
	order.UpdatedAt = updatedAt
	if filledAt.Valid {
		order.FilledAt = &filledAt.Time
	}
	if cancelledAt.Valid {
		order.CancelledAt = &cancelledAt.Time
	}
	order.StrategyID, _ = uuid.Parse(strategyID)

	return &order, nil
}

func (m *Manager) SavePosition(position *types.Position) error {
	if m.sqlite == nil {
		return fmt.Errorf("SQLite not available")
	}

	m.sqlite.mu.Lock()
	defer m.sqlite.mu.Unlock()

	query := `INSERT OR REPLACE INTO positions 
		(id, exchange, symbol, asset_type, side, quantity, avg_entry_price, current_price,
		unrealized_pnl, realized_pnl, roi, leverage, isolated_margin, liquidation_price,
		opened_at, updated_at, strategy_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := m.sqlite.db.Exec(query,
		position.ID.String(),
		string(position.Exchange),
		position.Symbol,
		string(position.AssetType),
		string(position.Side),
		position.Quantity.String(),
		position.AvgEntryPrice.String(),
		position.CurrentPrice.String(),
		position.UnrealizedPnL.String(),
		position.RealizedPnL.String(),
		position.ROI.String(),
		position.Leverage.String(),
		position.IsolatedMargin.String(),
		position.LiquidationPrice.String(),
		position.OpenedAt,
		position.UpdatedAt,
		position.StrategyID.String(),
	)

	return err
}

func (m *Manager) GetPositions() ([]*types.Position, error) {
	if m.sqlite == nil {
		return nil, fmt.Errorf("SQLite not available")
	}

	m.sqlite.mu.RLock()
	defer m.sqlite.mu.RUnlock()

	rows, err := m.sqlite.db.Query("SELECT * FROM positions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []*types.Position
	for rows.Next() {
		position, err := scanPosition(rows)
		if err != nil {
			continue
		}
		positions = append(positions, position)
	}

	return positions, nil
}

func scanPosition(rows *sql.Rows) (*types.Position, error) {
	var position types.Position
	var id, exchange, symbol, assetType, side string
	var quantity, avgEntryPrice, currentPrice, unrealizedPnL, realizedPnL, roi, leverage string
	var isolatedMargin, liquidationPrice string
	var openedAt, updatedAt time.Time
	var strategyID string

	err := rows.Scan(
		&id, &exchange, &symbol, &assetType, &side, &quantity, &avgEntryPrice,
		&currentPrice, &unrealizedPnL, &realizedPnL, &roi, &leverage,
		&isolatedMargin, &liquidationPrice, &openedAt, &updatedAt, &strategyID,
	)
	if err != nil {
		return nil, err
	}

	position.ID, _ = uuid.Parse(id)
	position.Exchange = types.Exchange(exchange)
	position.Symbol = symbol
	position.AssetType = types.AssetType(assetType)
	position.Side = types.PositionSide(side)
	position.Quantity, _ = decimal.NewFromString(quantity)
	position.AvgEntryPrice, _ = decimal.NewFromString(avgEntryPrice)
	position.CurrentPrice, _ = decimal.NewFromString(currentPrice)
	position.UnrealizedPnL, _ = decimal.NewFromString(unrealizedPnL)
	position.RealizedPnL, _ = decimal.NewFromString(realizedPnL)
	position.ROI, _ = decimal.NewFromString(roi)
	position.Leverage, _ = decimal.NewFromString(leverage)
	position.IsolatedMargin, _ = decimal.NewFromString(isolatedMargin)
	position.LiquidationPrice, _ = decimal.NewFromString(liquidationPrice)
	position.OpenedAt = openedAt
	position.UpdatedAt = updatedAt
	position.StrategyID, _ = uuid.Parse(strategyID)

	return &position, nil
}

func (m *Manager) SaveCandle(candle *types.Candle) error {
	if m.sqlite == nil {
		return fmt.Errorf("SQLite not available")
	}

	m.sqlite.mu.Lock()
	defer m.sqlite.mu.Unlock()

	query := `INSERT OR REPLACE INTO candles 
		(symbol, exchange, timeframe, timestamp, open, high, low, close, volume, 
		quote_volume, taker_buy_volume, trades, closed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := m.sqlite.db.Exec(query,
		candle.Symbol,
		string(candle.Exchange),
		candle.Timeframe,
		candle.Timestamp,
		candle.Open.String(),
		candle.High.String(),
		candle.Low.String(),
		candle.Close.String(),
		candle.Volume.String(),
		candle.QuoteVolume.String(),
		candle.TakerBuyVolume.String(),
		candle.Trades,
		candle.Closed,
	)

	return err
}

func (m *Manager) GetCandles(symbol, timeframe string, start, end time.Time, limit int) ([]*types.Candle, error) {
	if m.sqlite == nil {
		return nil, fmt.Errorf("SQLite not available")
	}

	m.sqlite.mu.RLock()
	defer m.sqlite.mu.RUnlock()

	query := "SELECT * FROM candles WHERE symbol = ? AND timeframe = ?"
	args := []interface{}{symbol, timeframe}

	if !start.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, start)
	}

	if !end.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, end)
	}

	query += " ORDER BY timestamp DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := m.sqlite.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candles []*types.Candle
	for rows.Next() {
		candle, err := scanCandle(rows)
		if err != nil {
			continue
		}
		candles = append(candles, candle)
	}

	return candles, nil
}

func scanCandle(rows *sql.Rows) (*types.Candle, error) {
	var candle types.Candle
	var symbol, exchange, timeframe string
	var timestamp time.Time
	var open, high, low, close, volume, quoteVolume, takerBuyVolume string
	var trades int64
	var closed bool

	err := rows.Scan(
		&symbol, &exchange, &timeframe, &timestamp,
		&open, &high, &low, &close, &volume,
		&quoteVolume, &takerBuyVolume, &trades, &closed,
	)
	if err != nil {
		return nil, err
	}

	candle.Symbol = symbol
	candle.Exchange = types.Exchange(exchange)
	candle.Timeframe = timeframe
	candle.Timestamp = timestamp
	candle.Open, _ = decimal.NewFromString(open)
	candle.High, _ = decimal.NewFromString(high)
	candle.Low, _ = decimal.NewFromString(low)
	candle.Close, _ = decimal.NewFromString(close)
	candle.Volume, _ = decimal.NewFromString(volume)
	candle.QuoteVolume, _ = decimal.NewFromString(quoteVolume)
	candle.TakerBuyVolume, _ = decimal.NewFromString(takerBuyVolume)
	candle.Trades = trades
	candle.Closed = closed

	return &candle, nil
}

func (m *Manager) SaveSignal(signal *types.Signal) error {
	if m.sqlite == nil {
		return fmt.Errorf("SQLite not available")
	}

	m.sqlite.mu.Lock()
	defer m.sqlite.mu.Unlock()

	query := `INSERT INTO signals 
		(id, strategy_id, symbol, exchange, action, strength, price, stop_loss, take_profit,
		quantity, reason, confidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := m.sqlite.db.Exec(query,
		signal.ID.String(),
		signal.StrategyID.String(),
		signal.Symbol,
		string(signal.Exchange),
		string(signal.Action),
		signal.Strength.String(),
		signal.Price.String(),
		signal.StopLoss.String(),
		signal.TakeProfit.String(),
		signal.Quantity.String(),
		signal.Reason,
		signal.Confidence.String(),
		signal.CreatedAt,
	)

	return err
}

func (m *Manager) SavePortfolioSnapshot(snapshot *types.Portfolio) error {
	if m.sqlite == nil {
		return fmt.Errorf("SQLite not available")
	}

	m.sqlite.mu.Lock()
	defer m.sqlite.mu.Unlock()

	query := `INSERT INTO portfolio_snapshots 
		(id, total_value, cash_balance, equity, margin_used, margin_available,
		unrealized_pnl, realized_pnl, day_pnl, snapshot_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := m.sqlite.db.Exec(query,
		uuid.New().String(),
		snapshot.TotalValue.String(),
		snapshot.CashBalance.String(),
		snapshot.Equity.String(),
		snapshot.MarginUsed.String(),
		snapshot.MarginAvailable.String(),
		snapshot.UnrealizedPnL.String(),
		snapshot.RealizedPnL.String(),
		snapshot.DayPnL.String(),
		time.Now(),
	)

	return err
}

func (m *Manager) GetCache(key string) (string, error) {
	if m.redis == nil {
		return "", fmt.Errorf("Redis not available")
	}
	return "", nil
}

func (m *Manager) SetCache(key, value string, ttl time.Duration) error {
	if m.redis == nil {
		return fmt.Errorf("Redis not available")
	}
	return nil
}

func (m *Manager) DeleteCache(key string) error {
	if m.redis == nil {
		return fmt.Errorf("Redis not available")
	}
	return nil
}

func (m *Manager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if m.sqlite != nil {
		m.sqlite.mu.RLock()
		defer m.sqlite.mu.RUnlock()

		var orderCount, positionCount, candleCount, signalCount int
		m.sqlite.db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&orderCount)
		m.sqlite.db.QueryRow("SELECT COUNT(*) FROM positions").Scan(&positionCount)
		m.sqlite.db.QueryRow("SELECT COUNT(*) FROM candles").Scan(&candleCount)
		m.sqlite.db.QueryRow("SELECT COUNT(*) FROM signals").Scan(&signalCount)

		stats["orders"] = orderCount
		stats["positions"] = positionCount
		stats["candles"] = candleCount
		stats["signals"] = signalCount
	}

	stats["sqlite"] = m.sqlite != nil
	stats["postgres"] = m.postgres != nil
	stats["redis"] = m.redis != nil

	return stats
}

func (m *Manager) BeginTx(ctx context.Context) error {
	return nil
}

func (m *Manager) CommitTx() error {
	return nil
}

func (m *Manager) RollbackTx() error {
	return nil
}
