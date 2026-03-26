package opentreder_test

import (
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigLoad(t *testing.T) {
	configContent := `
app:
  name: opentreder
  env: test
  log_level: debug
  data_dir: /tmp/opentreder

server:
  rest:
    host: 0.0.0.0
    port: 8080
    timeout: 30s
  websocket:
    host: 0.0.0.0
    port: 8081
  grpc:
    host: 0.0.0.0
    port: 8082

database:
  sqlite:
    path: /tmp/opentreder/opentreder.db
  postgres:
    enabled: false
    host: localhost
    port: 5432
    user: opentreder
    password: password
    database: opentreder

redis:
  host: localhost
  port: 6379
  password: ""
  db: 0

exchanges:
  binance:
    enabled: true
    api_key: test_key
    api_secret: test_secret
    testnet: true
  bybit:
    enabled: false

ai:
  enabled: true
  model: ensemble
  llm:
    provider: openai
    model: gpt-4
    api_key: test_key
`

	tempFile, err := os.CreateTemp("", "config_test_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(configContent)
	require.NoError(t, err)
	tempFile.Close()

	cfg, err := Load(tempFile.Name())
	require.NoError(t, err)

	assert.Equal(t, "opentreder", cfg.App.Name)
	assert.Equal(t, "test", cfg.App.Env)
	assert.Equal(t, "debug", cfg.App.LogLevel)

	assert.Equal(t, "0.0.0.0", cfg.Server.REST.Host)
	assert.Equal(t, 8080, cfg.Server.REST.Port)
	assert.Equal(t, 30*time.Second, cfg.Server.REST.Timeout)

	assert.Equal(t, "/tmp/opentreder/opentreder.db", cfg.Database.SQLite.Path)
	assert.False(t, cfg.Database.Postgres.Enabled)

	assert.True(t, cfg.Exchanges.Binance.Enabled)
	assert.Equal(t, "test_key", cfg.Exchanges.Binance.APIKey)
	assert.True(t, cfg.Exchanges.Binance.Testnet)

	assert.True(t, cfg.AI.Enabled)
	assert.Equal(t, "ensemble", cfg.AI.Model)
}

func TestConfigValidation(t *testing.T) {
	t.Run("Valid config", func(t *testing.T) {
		cfg := &Config{
			App: AppConfig{
				Name:    "opentreder",
				DataDir: "/tmp/opentreder",
			},
			Server: ServerConfig{
				REST: RESTConfig{
					Host: "0.0.0.0",
					Port: 8080,
				},
			},
		}

		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("Missing app name", func(t *testing.T) {
		cfg := &Config{
			App: AppConfig{
				Name: "",
			},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name")
	})

	t.Run("Invalid REST port", func(t *testing.T) {
		cfg := &Config{
			App: AppConfig{
				Name:    "opentreder",
				DataDir: "/tmp/opentreder",
			},
			Server: ServerConfig{
				REST: RESTConfig{
					Port: 0,
				},
			},
		}

		err := cfg.Validate()
		require.Error(t, err)
	})

	t.Run("Invalid port range", func(t *testing.T) {
		cfg := &Config{
			App: AppConfig{
				Name:    "opentreder",
				DataDir: "/tmp/opentreder",
			},
			Server: ServerConfig{
				REST: RESTConfig{
					Port: 70000,
				},
			},
		}

		err := cfg.Validate()
		require.Error(t, err)
	})
}

func TestExchangeConfig(t *testing.T) {
	cfg := &Config{
		Exchanges: ExchangesConfig{
			Binance: BinanceConfig{
				Enabled:  true,
				APIKey:   "key",
				APISecret: "secret",
				Testnet:  true,
			},
			Bybit: BybitConfig{
				Enabled:  true,
				APIKey:   "key",
				APISecret: "secret",
				Testnet:  true,
			},
		},
	}

	assert.True(t, cfg.Exchanges.Binance.Enabled)
	assert.True(t, cfg.Exchanges.Bybit.Testnet)
}

func TestRiskConfig(t *testing.T) {
	cfg := &Config{
		Risk: RiskConfig{
			MaxPositionSize:  decimal.NewFromFloat(0.1),
			MaxDrawdown:      decimal.NewFromFloat(0.2),
			MaxDailyLoss:     decimal.NewFromFloat(0.05),
			MaxLeverage:      decimal.NewFromFloat(10),
			StopLossPercent:  decimal.NewFromFloat(0.02),
			TakeProfitPercent: decimal.NewFromFloat(0.05),
		},
	}

	assert.True(t, cfg.Risk.MaxPositionSize.Equal(decimal.NewFromFloat(0.1)))
	assert.True(t, cfg.Risk.MaxDrawdown.Equal(decimal.NewFromFloat(0.2)))
	assert.True(t, cfg.Risk.MaxLeverage.Equal(decimal.NewFromFloat(10)))
}

func TestAIConfig(t *testing.T) {
	cfg := &Config{
		AI: AIConfig{
			Enabled: true,
			Model:   "ensemble",
			LLM: LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				APIKey:   "test_key",
				Temperature: 0.7,
				MaxTokens: 2000,
			},
			ML: MLConfig{
				ModelType: "xgboost",
				TrainingWindow: 1000,
				PredictionHorizon: 24,
			},
		},
	}

	assert.True(t, cfg.AI.Enabled)
	assert.Equal(t, "ensemble", cfg.AI.Model)
	assert.Equal(t, "openai", cfg.AI.LLM.Provider)
	assert.Equal(t, 0.7, cfg.AI.LLM.Temperature)
}

func TestDatabaseConfig(t *testing.T) {
	t.Run("SQLite only", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				SQLite: SQLiteConfig{
					Path: "/tmp/test.db",
				},
				Postgres: PostgresConfig{
					Enabled: false,
				},
			},
		}

		assert.NotEmpty(t, cfg.Database.SQLite.Path)
		assert.False(t, cfg.Database.Postgres.Enabled)
	})

	t.Run("Postgres enabled", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				SQLite: SQLiteConfig{
					Path: "/tmp/test.db",
				},
				Postgres: PostgresConfig{
					Enabled:  true,
					Host:     "localhost",
					Port:     5432,
					User:     "user",
					Password: "pass",
					Database: "testdb",
					SSLMode:  "disable",
				},
			},
		}

		assert.True(t, cfg.Database.Postgres.Enabled)
		assert.Equal(t, "localhost", cfg.Database.Postgres.Host)
	})
}

func TestLoadFromBytes(t *testing.T) {
	configContent := `
app:
  name: opentreder
  env: test
`

	cfg, err := LoadFromBytes([]byte(configContent))
	require.NoError(t, err)

	assert.Equal(t, "opentreder", cfg.App.Name)
	assert.Equal(t, "test", cfg.App.Env)
}

func TestConfigWithDefaults(t *testing.T) {
	cfg := NewDefaultConfig()

	assert.Equal(t, "opentreder", cfg.App.Name)
	assert.Equal(t, "development", cfg.App.Env)
	assert.Equal(t, "info", cfg.App.LogLevel)
	assert.Equal(t, 8080, cfg.Server.REST.Port)
	assert.Equal(t, 8081, cfg.Server.WebSocket.Port)
	assert.Equal(t, 8082, cfg.Server.GRPC.Port)
	assert.True(t, cfg.Risk.AutoStopLoss)
	assert.True(t, cfg.Risk.AutoTakeProfit)
}

func TestEnvironmentVariables(t *testing.T) {
	os.Setenv("OPENTRADER_LOG_LEVEL", "debug")
	defer os.Unsetenv("OPENTRADER_LOG_LEVEL")

	cfg := NewDefaultConfig()

	assert.Equal(t, "debug", cfg.App.LogLevel)
}

func TestConfigMerge(t *testing.T) {
	baseConfig := NewDefaultConfig()
	overrideConfig := &Config{
		App: AppConfig{
			Name:    "custom-name",
			LogLevel: "debug",
		},
	}

	merged := MergeConfig(baseConfig, overrideConfig)

	assert.Equal(t, "custom-name", merged.App.Name)
	assert.Equal(t, "debug", merged.App.LogLevel)
	assert.Equal(t, baseConfig.Server.REST.Port, merged.Server.REST.Port)
}
