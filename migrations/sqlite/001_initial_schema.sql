-- Migration: 001_initial_schema
-- Description: Create initial database schema

-- Create orders table
CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    client_order_id TEXT,
    exchange TEXT NOT NULL,
    symbol TEXT NOT NULL,
    asset_type TEXT NOT NULL DEFAULT 'crypto',
    market_type TEXT NOT NULL DEFAULT 'spot',
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
    metadata TEXT,
    tags TEXT
);

CREATE INDEX IF NOT EXISTS idx_orders_symbol ON orders(symbol);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_exchange ON orders(exchange);
CREATE INDEX IF NOT EXISTS idx_orders_created ON orders(created_at);
CREATE INDEX IF NOT EXISTS idx_orders_strategy ON orders(strategy_id);

-- Create positions table
CREATE TABLE IF NOT EXISTS positions (
    id TEXT PRIMARY KEY,
    exchange TEXT NOT NULL,
    symbol TEXT NOT NULL,
    asset_type TEXT NOT NULL DEFAULT 'crypto',
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
);

CREATE INDEX IF NOT EXISTS idx_positions_symbol ON positions(symbol);
CREATE INDEX IF NOT EXISTS idx_positions_exchange ON positions(exchange);
CREATE INDEX IF NOT EXISTS idx_positions_side ON positions(side);

-- Create trades table
CREATE TABLE IF NOT EXISTS trades (
    id TEXT PRIMARY KEY,
    exchange TEXT NOT NULL,
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    price TEXT NOT NULL,
    quantity TEXT NOT NULL,
    quote_qty TEXT,
    commission TEXT DEFAULT '0',
    commission_asset TEXT,
    timestamp DATETIME NOT NULL,
    order_id TEXT,
    is_buyer_maker INTEGER DEFAULT 0,
    fee_tier TEXT,
    realized_pnl TEXT
);

CREATE INDEX IF NOT EXISTS idx_trades_symbol ON trades(symbol);
CREATE INDEX IF NOT EXISTS idx_trades_timestamp ON trades(timestamp);
CREATE INDEX IF NOT EXISTS idx_trades_exchange ON trades(exchange);
CREATE INDEX IF NOT EXISTS idx_trades_order ON trades(order_id);

-- Create candles table
CREATE TABLE IF NOT EXISTS candles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
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
    UNIQUE(symbol, exchange, timeframe, timestamp)
);

CREATE INDEX IF NOT EXISTS idx_candles_symbol_timeframe ON candles(symbol, exchange, timeframe);
CREATE INDEX IF NOT EXISTS idx_candles_timestamp ON candles(timestamp);

-- Create strategies table
CREATE TABLE IF NOT EXISTS strategies (
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
);

CREATE INDEX IF NOT EXISTS idx_strategies_type ON strategies(type);
CREATE INDEX IF NOT EXISTS idx_strategies_enabled ON strategies(enabled);

-- Create signals table
CREATE TABLE IF NOT EXISTS signals (
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
    metadata TEXT,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_signals_symbol ON signals(symbol);
CREATE INDEX IF NOT EXISTS idx_signals_created ON signals(created_at);
CREATE INDEX IF NOT EXISTS idx_signals_strategy ON signals(strategy_id);

-- Create balances table
CREATE TABLE IF NOT EXISTS balances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    asset TEXT NOT NULL,
    exchange TEXT NOT NULL,
    free TEXT DEFAULT '0',
    locked TEXT DEFAULT '0',
    total TEXT DEFAULT '0',
    usd_value TEXT DEFAULT '0',
    updated_at DATETIME NOT NULL,
    UNIQUE(asset, exchange)
);

CREATE INDEX IF NOT EXISTS idx_balances_exchange ON balances(exchange);

-- Create portfolio snapshots table
CREATE TABLE IF NOT EXISTS portfolio_snapshots (
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
);

CREATE INDEX IF NOT EXISTS idx_portfolio_time ON portfolio_snapshots(snapshot_time);

-- Create exchanges table
CREATE TABLE IF NOT EXISTS exchanges (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    api_key TEXT,
    api_secret_encrypted TEXT,
    testnet INTEGER DEFAULT 0,
    enabled INTEGER DEFAULT 1,
    status TEXT DEFAULT 'disconnected',
    last_connected_at DATETIME,
    config TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- Create risk limits table
CREATE TABLE IF NOT EXISTS risk_limits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    max_value TEXT NOT NULL,
    current_value TEXT DEFAULT '0',
    enabled INTEGER DEFAULT 1,
    action TEXT,
    updated_at DATETIME NOT NULL
);

-- Create backtest results table
CREATE TABLE IF NOT EXISTS backtest_results (
    id TEXT PRIMARY KEY,
    strategy_name TEXT NOT NULL,
    symbol TEXT NOT NULL,
    timeframe TEXT,
    start_date DATETIME NOT NULL,
    end_date DATETIME NOT NULL,
    initial_balance TEXT NOT NULL,
    final_balance TEXT NOT NULL,
    total_return TEXT,
    sharpe_ratio TEXT,
    max_drawdown TEXT,
    win_rate TEXT,
    profit_factor TEXT,
    total_trades INTEGER,
    parameters TEXT,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_backtest_strategy ON backtest_results(strategy_name);
CREATE INDEX IF NOT EXISTS idx_backtest_created ON backtest_results(created_at);

-- Create AI cache table
CREATE TABLE IF NOT EXISTS ai_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key_hash TEXT NOT NULL UNIQUE,
    prompt TEXT NOT NULL,
    response TEXT,
    model TEXT,
    tokens_used INTEGER,
    created_at DATETIME NOT NULL,
    expires_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_ai_cache_expires ON ai_cache(expires_at);

-- Create notifications log table
CREATE TABLE IF NOT EXISTS notifications_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    channel TEXT NOT NULL,
    recipient TEXT,
    title TEXT NOT NULL,
    body TEXT,
    status TEXT DEFAULT 'pending',
    sent_at DATETIME,
    error TEXT,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications_log(status);
CREATE INDEX IF NOT EXISTS idx_notifications_created ON notifications_log(created_at);

-- Create API keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    secret_hash TEXT,
    permissions TEXT,
    rate_limit INTEGER DEFAULT 100,
    enabled INTEGER DEFAULT 1,
    last_used_at DATETIME,
    expires_at DATETIME,
    created_at DATETIME NOT NULL
);

-- Create sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    token TEXT NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

-- Insert default risk limits
INSERT OR IGNORE INTO risk_limits (name, type, max_value, enabled, action) VALUES
('Max Position Size', 'position', '1.0', 1, 'reject'),
('Max Order Size', 'order', '0.5', 1, 'reject'),
('Max Daily Loss', 'daily', '0.1', 1, 'stop_all'),
('Max Drawdown', 'equity', '0.2', 1, 'stop_all'),
('Max Exposure', 'exposure', '0.8', 1, 'reject'),
('Max Leverage', 'margin', '3', 1, 'reject'),
('Min Account Balance', 'balance', '100', 1, 'notify');

-- Create triggers for updated_at
CREATE TRIGGER IF NOT EXISTS update_orders_timestamp 
AFTER UPDATE ON orders
BEGIN
    UPDATE orders SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_positions_timestamp 
AFTER UPDATE ON positions
BEGIN
    UPDATE positions SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS update_balances_timestamp 
AFTER UPDATE ON balances
BEGIN
    UPDATE balances SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
