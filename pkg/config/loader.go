package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/shopspring/decimal"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

type Config struct {
	App         AppConfig         `mapstructure:"app"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Cache       CacheConfig       `mapstructure:"cache"`
	Trading     TradingConfig     `mapstructure:"trading"`
	Risk        RiskConfig        `mapstructure:"risk"`
	AI          AIConfig          `mapstructure:"ai"`
	UI          UIConfig          `mapstructure:"ui"`
	API         APIConfig         `mapstructure:"api"`
	WebSocket   WebSocketConfig   `mapstructure:"websocket"`
	Exchanges   ExchangesConfig   `mapstructure:"exchanges"`
	Strategies  StrategiesConfig  `mapstructure:"strategies"`
	Notifications NotificationsConfig `mapstructure:"notifications"`
	Security    SecurityConfig    `mapstructure:"security"`
	Logging     logger.LoggerConfig `mapstructure:"logging"`
	Metrics     MetricsConfig     `mapstructure:"metrics"`
}

type AppConfig struct {
	Name        string `mapstructure:"name"`
	Version     string `mapstructure:"version"`
	Environment string `mapstructure:"environment"`
	Debug       bool   `mapstructure:"debug"`
	DataDir     string `mapstructure:"data_dir"`
	ConfigDir   string `mapstructure:"config_dir"`
	LogDir      string `mapstructure:"log_dir"`
	PIDFile     string `mapstructure:"pid_file"`
	AutoReload  bool   `mapstructure:"auto_reload"`
}

type DatabaseConfig struct {
	SQLite      SQLiteConfig      `mapstructure:"sqlite"`
	PostgreSQL  PostgreSQLConfig  `mapstructure:"postgresql"`
	Mode        string            `mapstructure:"mode"`
	MaxOpenConns int              `mapstructure:"max_open_conns"`
	MaxIdleConns int              `mapstructure:"max_idle_conns"`
	ConnMaxLife  string           `mapstructure:"conn_max_life"`
	SSLMode     string            `mapstructure:"ssl_mode"`
}

type SQLiteConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	Path        string `mapstructure:"path"`
	JournalMode string `mapstructure:"journal_mode"`
	WALMode     bool   `mapstructure:"wal_mode"`
	CacheSize   int64  `mapstructure:"cache_size"`
}

type PostgreSQLConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	User       string `mapstructure:"user"`
	Password   string `mapstructure:"password"`
	Database   string `mapstructure:"database"`
	Schema     string `mapstructure:"schema"`
	SSLMode    string `mapstructure:"ssl_mode"`
	SSLCert    string `mapstructure:"ssl_cert"`
	SSLKey     string `mapstructure:"ssl_key"`
	SSLRootCert string `mapstructure:"ssl_root_cert"`
}

type CacheConfig struct {
	Redis     RedisConfig    `mapstructure:"redis"`
	Memory    MemoryCacheConfig `mapstructure:"memory"`
	Enabled   bool           `mapstructure:"enabled"`
	TTL       string         `mapstructure:"ttl"`
	MaxSize   int            `mapstructure:"max_size"`
}

type RedisConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Password   string `mapstructure:"password"`
	Database   int    `mapstructure:"database"`
	PoolSize   int    `mapstructure:"pool_size"`
	MinIdleConns int  `mapstructure:"min_idle_conns"`
	MaxRetries int    `mapstructure:"max_retries"`
	DialTimeout string `mapstructure:"dial_timeout"`
	ReadTimeout string `mapstructure:"read_timeout"`
	WriteTimeout string `mapstructure:"write_timeout"`
	ClusterMode bool  `mapstructure:"cluster_mode"`
	Nodes      []string `mapstructure:"nodes"`
}

type MemoryCacheConfig struct {
	Enabled bool `mapstructure:"enabled"`
	MaxSize int  `mapstructure:"max_size"`
	TTL     string `mapstructure:"ttl"`
}

type TradingConfig struct {
	Mode             string            `mapstructure:"mode"`
	Enabled          bool              `mapstructure:"enabled"`
	MaxOpenOrders    int               `mapstructure:"max_open_orders"`
	OrderTimeout     string            `mapstructure:"order_timeout"`
	RetryAttempts    int               `mapstructure:"retry_attempts"`
	RetryDelay       string            `mapstructure:"retry_delay"`
	SlippageTolerance decimal.Decimal  `mapstructure:"slippage_tolerance"`
	CommissionRate   decimal.Decimal   `mapstructure:"commission_rate"`
	DefaultLeverage  decimal.Decimal   `mapstructure:"default_leverage"`
	AllowMargin     bool              `mapstructure:"allow_margin"`
	AutoLiquidate   bool              `mapstructure:"auto_liquidate"`
	PartialFillEnabled bool           `mapstructure:"partial_fill_enabled"`
	IcebergEnabled  bool              `mapstructure:"iceberg_enabled"`
}

type RiskConfig struct {
	Enabled            bool             `mapstructure:"enabled"`
	MaxPositionSize   decimal.Decimal  `mapstructure:"max_position_size"`
	MaxOrderSize      decimal.Decimal  `mapstructure:"max_order_size"`
	MaxDailyLoss      decimal.Decimal  `mapstructure:"max_daily_loss"`
	MaxDrawdown       decimal.Decimal  `mapstructure:"max_drawdown"`
	MaxExposure       decimal.Decimal  `mapstructure:"max_exposure"`
	MaxLeverage       decimal.Decimal  `mapstructure:"max_leverage"`
	MinAccountBalance decimal.Decimal  `mapstructure:"min_account_balance"`
	PositionSizing    string           `mapstructure:"position_sizing"`
	StopLoss          decimal.Decimal  `mapstructure:"stop_loss"`
	TakeProfit        decimal.Decimal  `mapstructure:"take_profit"`
	TrailingStop      decimal.Decimal  `mapstructure:"trailing_stop"`
	RiskPerTrade      decimal.Decimal  `mapstructure:"risk_per_trade"`
}

type AIConfig struct {
	Enabled       bool           `mapstructure:"enabled"`
	Provider      string         `mapstructure:"provider"`
	Model         string         `mapstructure:"model"`
	APIKey        string         `mapstructure:"api_key"`
	APIEndpoint   string         `mapstructure:"api_endpoint"`
	MaxTokens     int            `mapstructure:"max_tokens"`
	Temperature   float64        `mapstructure:"temperature"`
	TopP          float64        `mapstructure:"top_p"`
	CacheEnabled  bool           `mapstructure:"cache_enabled"`
	CacheTTL      string         `mapstructure:"cache_ttl"`
	Timeout       string         `mapstructure:"timeout"`
	RetryAttempts int            `mapstructure:"retry_attempts"`
	ML            MLConfig        `mapstructure:"ml"`
	Sentiment     SentimentConfig `mapstructure:"sentiment"`
}

type MLConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	ModelPath      string `mapstructure:"model_path"`
	TrainInterval  string `mapstructure:"train_interval"`
	LookbackPeriod  int   `mapstructure:"lookback_period"`
	PredictionHorizon int `mapstructure:"prediction_horizon"`
	Features       []string `mapstructure:"features"`
	Models         []string `mapstructure:"models"`
}

type SentimentConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	Sources       []string `mapstructure:"sources"`
	UpdateInterval string   `mapstructure:"update_interval"`
	Weight        float64  `mapstructure:"weight"`
}

type UIConfig struct {
	Theme        string       `mapstructure:"theme"`
	ColorScheme  string       `mapstructure:"color_scheme"`
	RefreshRate  int          `mapstructure:"refresh_rate"`
	ShowAdvanced bool         `mapstructure:"show_advanced"`
	Dashboard    DashboardConfig `mapstructure:"dashboard"`
	Table        TableConfig     `mapstructure:"table"`
	Chart        ChartConfig     `mapstructure:"chart"`
}

type DashboardConfig struct {
	ShowPortfolio   bool `mapstructure:"show_portfolio"`
	ShowPositions  bool `mapstructure:"show_positions"`
	ShowOrders     bool `mapstructure:"show_orders"`
	ShowTrades     bool `mapstructure:"show_trades"`
	ShowCharts     bool `mapstructure:"show_charts"`
	ShowSignals    bool `mapstructure:"show_signals"`
	ShowRisk       bool `mapstructure:"show_risk"`
	ShowPnL        bool `mapstructure:"show_pnl"`
}

type TableConfig struct {
	PageSize    int  `mapstructure:"page_size"`
	Sortable    bool `mapstructure:"sortable"`
	Filterable  bool `mapstructure:"filterable"`
	Exportable  bool `mapstructure:"exportable"`
	ColResize   bool `mapstructure:"col_resize"`
}

type ChartConfig struct {
	Type        string `mapstructure:"type"`
	Timeframes  []string `mapstructure:"timeframes"`
	Indicators  []string `mapstructure:"indicators"`
	ShowVolume  bool     `mapstructure:"show_volume"`
	ShowGrid    bool     `mapstructure:"show_grid"`
}

type APIConfig struct {
	Enabled       bool          `mapstructure:"enabled"`
	Host          string        `mapstructure:"host"`
	Port          int           `mapstructure:"port"`
	TLSEnabled    bool          `mapstructure:"tls_enabled"`
	TLSCert       string        `mapstructure:"tls_cert"`
	TLSKey        string        `mapstructure:"tls_key"`
	CORSEnabled   bool          `mapstructure:"cors_enabled"`
	CORSOrigins   []string      `mapstructure:"cors_origins"`
	RateLimit     RateLimitConfig `mapstructure:"rate_limit"`
	Auth          AuthConfig      `mapstructure:"auth"`
}

type RateLimitConfig struct {
	Enabled       bool `mapstructure:"enabled"`
	RequestsPerSec int `mapstructure:"requests_per_sec"`
	Burst         int  `mapstructure:"burst"`
}

type AuthConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	Type          string `mapstructure:"type"`
	JWTExpiration string `mapstructure:"jwt_expiration"`
	JWTSecret     string `mapstructure:"jwt_secret"`
}

type WebSocketConfig struct {
	Enabled      bool             `mapstructure:"enabled"`
	Host         string           `mapstructure:"host"`
	Port         int              `mapstructure:"port"`
	TLSEnabled   bool             `mapstructure:"tls_enabled"`
	TLSCert      string           `mapstructure:"tls_cert"`
	TLSKey       string           `mapstructure:"tls_key"`
	ReadBuffer   int              `mapstructure:"read_buffer"`
	WriteBuffer  int              `mapstructure:"write_buffer"`
	PingInterval string           `mapstructure:"ping_interval"`
	PongTimeout  string           `mapstructure:"pong_timeout"`
}

type ExchangesConfig struct {
	Binance         BinanceConfig         `mapstructure:"binance"`
	Bybit           BybitConfig           `mapstructure:"bybit"`
	Coinbase        CoinbaseConfig        `mapstructure:"coinbase"`
	Kraken          KrakenConfig          `mapstructure:"kraken"`
	OKX             OKXConfig             `mapstructure:"okx"`
	Alpaca          AlpacaConfig          `mapstructure:"alpaca"`
	InteractiveBrokers InteractiveBrokersConfig `mapstructure:"interactive_brokers"`
	OANDA           OANDAConfig           `mapstructure:"oanda"`
	Enabled         []string              `mapstructure:"enabled"`
}

type BinanceConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	APIKey            string   `mapstructure:"api_key"`
	APISecret         string   `mapstructure:"api_secret"`
	Testnet           bool     `mapstructure:"testnet"`
	BaseURL           string   `mapstructure:"base_url"`
	WSURL             string   `mapstructure:"ws_url"`
	RecvWindow        int      `mapstructure:"recv_window"`
	MaxRetries        int      `mapstructure:"max_retries"`
	RateLimitPerSec   int      `mapstructure:"rate_limit_per_sec"`
}

type BybitConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	APIKey      string `mapstructure:"api_key"`
	APISecret   string `mapstructure:"api_secret"`
	Testnet     bool   `mapstructure:"testnet"`
	BaseURL     string `mapstructure:"base_url"`
	WSURL       string `mapstructure:"ws_url"`
	Category    string `mapstructure:"category"`
}

type CoinbaseConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	APIKey        string `mapstructure:"api_key"`
	APISecret     string `mapstructure:"api_secret"`
	Sandbox       bool   `mapstructure:"sandbox"`
	BaseURL       string `mapstructure:"base_url"`
}

type KrakenConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	APIKey    string `mapstructure:"api_key"`
	APISecret string `mapstructure:"api_secret"`
}

type OKXConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	APIKey    string `mapstructure:"api_key"`
	APISecret string `mapstructure:"api_secret"`
	Passphrase string `mapstructure:"passphrase"`
	Simnet    bool   `mapstructure:"simnet"`
}

type AlpacaConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	APIKey       string `mapstructure:"api_key"`
	APISecret    string `mapstructure:"api_secret"`
	PaperTrading bool   `mapstructure:"paper_trading"`
	BaseURL      string `mapstructure:"base_url"`
}

type InteractiveBrokersConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	Host          string `mapstructure:"host"`
	Port          int    `mapstructure:"port"`
	ClientID      int    `mapstructure:"client_id"`
}

type OANDAConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	APIKey      string `mapstructure:"api_key"`
	AccountID   string `mapstructure:"account_id"`
	Environment string `mapstructure:"environment"`
}

type StrategiesConfig struct {
	Enabled   []string            `mapstructure:"enabled"`
	Directory string              `mapstructure:"directory"`
	Params    map[string]map[string]interface{} `mapstructure:"params"`
}

type NotificationsConfig struct {
	Enabled    bool               `mapstructure:"enabled"`
	Telegram   TelegramConfig     `mapstructure:"telegram"`
	Slack      SlackConfig        `mapstructure:"slack"`
	Email      EmailConfig        `mapstructure:"email"`
	Discord    DiscordConfig      `mapstructure:"discord"`
	Webhook    WebhookConfig      `mapstructure:"webhook"`
}

type TelegramConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	BotToken     string   `mapstructure:"bot_token"`
	ChatIDs      []string `mapstructure:"chat_ids"`
	ParseMode    string   `mapstructure:"parse_mode"`
	DisableNotification bool `mapstructure:"disable_notification"`
}

type SlackConfig struct {
	Enabled    bool     `mapstructure:"enabled"`
	WebhookURL string   `mapstructure:"webhook_url"`
	Channel    string   `mapstructure:"channel"`
	Username   string   `mapstructure:"username"`
	IconEmoji  string   `mapstructure:"icon_emoji"`
}

type EmailConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	SMTPHost      string `mapstructure:"smtp_host"`
	SMTPPort      int    `mapstructure:"smtp_port"`
	Username      string `mapstructure:"username"`
	Password      string `mapstructure:"password"`
	From          string `mapstructure:"from"`
	To            []string `mapstructure:"to"`
	UseTLS        bool   `mapstructure:"use_tls"`
}

type DiscordConfig struct {
	Enabled    bool     `mapstructure:"enabled"`
	WebhookURL string   `mapstructure:"webhook_url"`
	Username   string   `mapstructure:"username"`
	AvatarURL  string   `mapstructure:"avatar_url"`
}

type WebhookConfig struct {
	Enabled    bool              `mapstructure:"enabled"`
	URLs       map[string]string `mapstructure:"urls"`
}

type SecurityConfig struct {
	Encryption    EncryptionConfig    `mapstructure:"encryption"`
	Vault         VaultConfig         `mapstructure:"vault"`
	AllowedHosts  []string            `mapstructure:"allowed_hosts"`
	AllowedOrigins []string           `mapstructure:"allowed_origins"`
}

type EncryptionConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Key      string `mapstructure:"key"`
	Algorithm string `mapstructure:"algorithm"`
}

type VaultConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Address    string `mapstructure:"address"`
	Token      string `mapstructure:"token"`
	PathPrefix string `mapstructure:"path_prefix"`
}

type MetricsConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Path         string `mapstructure:"path"`
	ExportInterval string `mapstructure:"export_interval"`
}

var (
	config     *Config
	configOnce sync.Once
	configMu   sync.RWMutex
)

func Load(configPath string, opts ...Option) (*Config, error) {
	var loadErr error

	configOnce.Do(func() {
		v := viper.New()

		if configPath != "" {
			v.SetConfigFile(configPath)
		} else {
			v.SetConfigName("config")
			v.SetConfigType("yaml")

			homeDir, err := os.UserHomeDir()
			if err == nil {
				v.AddConfigPath(filepath.Join(homeDir, ".opentreder"))
				v.AddConfigPath("./configs")
				v.AddConfigPath("/etc/opentreder")
			}
			v.AddConfigPath(".")
		}

		v.SetEnvPrefix("OPENTRADER")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		defaultConfig(v)

		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				loadErr = fmt.Errorf("failed to read config: %w", err)
				return
			}
			logger.Warnf("No config file found, using defaults: %v", err)
		}

		config = &Config{}
		if err := v.Unmarshal(config, opts...); err != nil {
			loadErr = fmt.Errorf("failed to unmarshal config: %w", err)
			return
		}

		if err := validateConfig(config); err != nil {
			loadErr = fmt.Errorf("config validation failed: %w", err)
			return
		}
	})

	return config, loadErr
}

func Get() *Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return config
}

func Set(cfg *Config) {
	configMu.Lock()
	defer configMu.Unlock()
	config = cfg
}

func defaultConfig(v *viper.Viper) {
	v.SetDefault("app.name", "OpenTrader")
	v.SetDefault("app.version", "1.0.0")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.debug", false)
	v.SetDefault("app.data_dir", "./data")
	v.SetDefault("app.config_dir", "./configs")
	v.SetDefault("app.log_dir", "./logs")

	v.SetDefault("database.mode", "sqlite")
	v.SetDefault("database.sqlite.enabled", true)
	v.SetDefault("database.sqlite.path", "./data/opentreder.db")
	v.SetDefault("database.sqlite.journal_mode", "WAL")
	v.SetDefault("database.sqlite.wal_mode", true)
	v.SetDefault("database.sqlite.cache_size", 10000)

	v.SetDefault("database.postgresql.enabled", false)
	v.SetDefault("database.postgresql.host", "localhost")
	v.SetDefault("database.postgresql.port", 5432)
	v.SetDefault("database.postgresql.database", "opentreder")
	v.SetDefault("database.postgresql.schema", "public")
	v.SetDefault("database.postgresql.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_life", "1h")

	v.SetDefault("cache.enabled", true)
	v.SetDefault("cache.ttl", "5m")
	v.SetDefault("cache.max_size", 1000)

	v.SetDefault("cache.redis.enabled", false)
	v.SetDefault("cache.redis.host", "localhost")
	v.SetDefault("cache.redis.port", 6379)
	v.SetDefault("cache.redis.database", 0)
	v.SetDefault("cache.redis.pool_size", 10)
	v.SetDefault("cache.redis.min_idle_conns", 5)
	v.SetDefault("cache.redis.max_retries", 3)
	v.SetDefault("cache.redis.dial_timeout", "5s")
	v.SetDefault("cache.redis.read_timeout", "3s")
	v.SetDefault("cache.redis.write_timeout", "3s")

	v.SetDefault("trading.enabled", true)
	v.SetDefault("trading.mode", "paper")
	v.SetDefault("trading.max_open_orders", 100)
	v.SetDefault("trading.order_timeout", "30s")
	v.SetDefault("trading.retry_attempts", 3)
	v.SetDefault("trading.retry_delay", "1s")
	v.SetDefault("trading.slippage_tolerance", "0.001")
	v.SetDefault("trading.commission_rate", "0.001")
	v.SetDefault("trading.default_leverage", "1")

	v.SetDefault("risk.enabled", true)
	v.SetDefault("risk.max_position_size", "1.0")
	v.SetDefault("risk.max_order_size", "0.5")
	v.SetDefault("risk.max_daily_loss", "0.1")
	v.SetDefault("risk.max_drawdown", "0.2")
	v.SetDefault("risk.max_exposure", "0.8")
	v.SetDefault("risk.max_leverage", "3")
	v.SetDefault("risk.min_account_balance", "100")
	v.SetDefault("risk.risk_per_trade", "0.02")

	v.SetDefault("ai.enabled", false)
	v.SetDefault("ai.provider", "openai")
	v.SetDefault("ai.model", "gpt-4")
	v.SetDefault("ai.max_tokens", 2048)
	v.SetDefault("ai.temperature", 0.7)
	v.SetDefault("ai.top_p", 0.9)
	v.SetDefault("ai.cache_enabled", true)
	v.SetDefault("ai.cache_ttl", "24h")
	v.SetDefault("ai.timeout", "60s")
	v.SetDefault("ai.retry_attempts", 3)

	v.SetDefault("ui.theme", "dark")
	v.SetDefault("ui.refresh_rate", 1000)
	v.SetDefault("ui.show_advanced", false)
	v.SetDefault("ui.dashboard.show_portfolio", true)
	v.SetDefault("ui.dashboard.show_positions", true)
	v.SetDefault("ui.dashboard.show_orders", true)
	v.SetDefault("ui.dashboard.show_trades", true)
	v.SetDefault("ui.dashboard.show_charts", true)
	v.SetDefault("ui.dashboard.show_signals", true)
	v.SetDefault("ui.dashboard.show_risk", true)
	v.SetDefault("ui.dashboard.show_pnl", true)
	v.SetDefault("ui.table.page_size", 25)
	v.SetDefault("ui.table.sortable", true)
	v.SetDefault("ui.table.filterable", true)
	v.SetDefault("ui.chart.type", "candlestick")
	v.SetDefault("ui.chart.show_volume", true)
	v.SetDefault("ui.chart.show_grid", true)

	v.SetDefault("api.enabled", true)
	v.SetDefault("api.host", "0.0.0.0")
	v.SetDefault("api.port", 8080)
	v.SetDefault("api.tls_enabled", false)
	v.SetDefault("api.cors_enabled", true)
	v.SetDefault("api.rate_limit.enabled", true)
	v.SetDefault("api.rate_limit.requests_per_sec", 100)
	v.SetDefault("api.rate_limit.burst", 200)
	v.SetDefault("api.auth.enabled", false)
	v.SetDefault("api.auth.type", "jwt")
	v.SetDefault("api.auth.jwt_expiration", "24h")

	v.SetDefault("websocket.enabled", true)
	v.SetDefault("websocket.host", "0.0.0.0")
	v.SetDefault("websocket.port", 8081)
	v.SetDefault("websocket.ping_interval", "30s")
	v.SetDefault("websocket.pong_timeout", "60s")
	v.SetDefault("websocket.read_buffer", 1024)
	v.SetDefault("websocket.write_buffer", 1024)

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "./logs/opentreder.log")
	v.SetDefault("logging.report_caller", true)
	v.SetDefault("logging.timestamp_format", "2006-01-02T15:04:05Z07:00")
	v.SetDefault("logging.max_size", 100)
	v.SetDefault("logging.max_backups", 30)
	v.SetDefault("logging.max_age", 90)
	v.SetDefault("logging.compress", true)
	v.SetDefault("logging.enable_file_log", true)
	v.SetDefault("logging.enable_console_log", true)

	v.SetDefault("metrics.enabled", false)
	v.SetDefault("metrics.host", "0.0.0.0")
	v.SetDefault("metrics.port", 9090)
	v.SetDefault("metrics.path", "/metrics")
	v.SetDefault("metrics.export_interval", "15s")
}

func validateConfig(cfg *Config) error {
	if cfg.App.Name == "" {
		return fmt.Errorf("app name is required")
	}

	if cfg.App.DataDir == "" {
		return fmt.Errorf("data directory is required")
	}

	if err := os.MkdirAll(cfg.App.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	if cfg.Trading.SlippageTolerance.LessThan(decimal.Zero) || cfg.Trading.SlippageTolerance.GreaterThan(decimal.NewFromInt(1)) {
		return fmt.Errorf("slippage tolerance must be between 0 and 1")
	}

	if cfg.Risk.MaxLeverage.LessThan(decimal.One) {
		return fmt.Errorf("max leverage must be at least 1")
	}

	return nil
}

func (c *Config) GetString(path string) string {
	v := viper.GetString(path)
	if v == nil {
		return ""
	}
	return v.(string)
}

func (c *Config) GetInt(path string) int {
	return viper.GetInt(path)
}

func (c *Config) GetBool(path string) bool {
	return viper.GetBool(path)
}

func (c *Config) GetFloat64(path string) float64 {
	return viper.GetFloat64(path)
}

func (c *Config) GetStringSlice(path string) []string {
	return viper.GetStringSlice(path)
}

func (c *Config) GetStringMap(path string) map[string]interface{} {
	return viper.GetStringMap(path)
}

type Option func(*viper.Viper)

func (c *Config) IsSet(key string) bool {
	return viper.IsSet(key)
}

func (c *Config) AllKeys() []string {
	return viper.AllKeys()
}

func (c *Config) AllSettings() map[string]interface{} {
	return viper.AllSettings()
}

func BindEnv(keys ...string) {
	viper.BindEnv(keys...)
}

func Set(key string, value interface{}) {
	viper.Set(key, value)
}

func WriteConfig(path string) error {
	return viper.WriteConfigAs(path)
}

func (c *Config) Sub(key string) (*Config, error) {
	sub := viper.Sub(key)
	if sub == nil {
		return nil, fmt.Errorf("config key not found: %s", key)
	}

	result := &Config{}
	if err := sub.Unmarshal(result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Config) Merge(override map[string]interface{}) error {
	for key, value := range override {
		if err := viper.Set(key, value); err != nil {
			return err
		}
	}

	if err := viper.Unmarshal(c); err != nil {
		return err
	}

	return validateConfig(c)
}

func (c *Config) String() string {
	var result strings.Builder
	t := reflect.TypeOf(*c)
	v := reflect.ValueOf(*c)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		if field.PkgPath != "" {
			continue
		}

		switch field.Type.Kind() {
		case reflect.Struct:
			nested := v.Field(i).Interface()
			result.WriteString(fmt.Sprintf("%s:\n%s\n", field.Name, structToString(nested)))
		default:
			result.WriteString(fmt.Sprintf("%s: %v\n", field.Name, value.Interface()))
		}
	}

	return result.String()
}

func structToString(v interface{}) string {
	var result strings.Builder
	t := reflect.TypeOf(v)
	vals := reflect.ValueOf(v)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := vals.Field(i)

		if field.PkgPath != "" {
			continue
		}

		switch field.Type.Kind() {
		case reflect.Struct:
			result.WriteString(fmt.Sprintf("  %s:\n%s", field.Name, indentString(structToString(value.Interface()), "    ")))
		case reflect.Slice:
			if value.Len() > 0 {
				result.WriteString(fmt.Sprintf("  %s: [\n", field.Name))
				for j := 0; j < value.Len(); j++ {
					result.WriteString(fmt.Sprintf("    %v,\n", value.Index(j).Interface()))
				}
				result.WriteString("  ]\n")
			}
		case reflect.Map:
			result.WriteString(fmt.Sprintf("  %s: %v\n", field.Name, value.Interface()))
		default:
			result.WriteString(fmt.Sprintf("  %s: %v\n", field.Name, value.Interface()))
		}
	}

	return result.String()
}

func indentString(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

func (c *Config) Clone() *Config {
	clone := &Config{}
	t := reflect.TypeOf(*c)
	v := reflect.ValueOf(*c)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		if field.PkgPath != "" {
			continue
		}

		dest := reflect.NewAt(field.Type, reflect.ValueOf(clone).Field(i).Addr().UnsafePointer())
		dest.Elem().Set(value)
	}

	return clone
}

func ExportAPIEndpoint(exchange types.Exchange, endpoint string) string {
	cfg := Get()
	if cfg == nil {
		return ""
	}

	switch exchange {
	case types.ExchangeBinance:
		if cfg.Exchanges.Binance.Testnet {
			return cfg.Exchanges.Binance.BaseURL + endpoint
		}
		return "https://api.binance.com" + endpoint
	case types.ExchangeBybit:
		if cfg.Exchanges.Bybit.Testnet {
			return cfg.Exchanges.Bybit.BaseURL + endpoint
		}
		return "https://api.bybit.com" + endpoint
	default:
		return endpoint
	}
}

func GetExchangeConfig(exchange types.Exchange) interface{} {
	cfg := Get()
	if cfg == nil {
		return nil
	}

	switch exchange {
	case types.ExchangeBinance:
		return cfg.Exchanges.Binance
	case types.ExchangeBybit:
		return cfg.Exchanges.Bybit
	case types.ExchangeCoinbase:
		return cfg.Exchanges.Coinbase
	case types.ExchangeKraken:
		return cfg.Exchanges.Kraken
	case types.ExchangeOKX:
		return cfg.Exchanges.OKX
	case types.ExchangeAlpaca:
		return cfg.Exchanges.Alpaca
	case types.ExchangeOANDA:
		return cfg.Exchanges.OANDA
	default:
		return nil
	}
}

func castToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%f", val)
	case bool:
		return fmt.Sprintf("%t", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func castToInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		i, _ := cast.ToInt(val)
		return i
	default:
		return 0
	}
}

func castToFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := cast.ToFloat64(val)
		return f
	default:
		return 0
	}
}

func castToBool(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case int:
		return val != 0
	case string:
		b, _ := cast.ToBool(val)
		return b
	default:
		return false
	}
}

func castToStringSlice(v interface{}) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		result := make([]string, len(val))
		for i, v := range val {
			result[i] = castToString(v)
		}
		return result
	default:
		return nil
	}
}
