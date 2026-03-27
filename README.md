# OpenTrader 🚀

### Enterprise-Grade AI-Powered Trading Framework (10x More Powerful than NautilusTrader)

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](https://golang.org/)
[![Build Status](https://github.com/Nirmal09809/opentreder/actions/workflows/ci.yml/badge.svg)](https://github.com/Nirmal09809/opentreder/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nirmal09809/opentreder)](https://goreportcard.com/report/github.com/Nirmal09809/opentreder)

---

## ✨ Features

### 🤖 AI-Powered Trading
- **LLM Integration** - GPT-4 powered market analysis and decision making
- **Machine Learning Models** - XGBoost, LSTM, Transformer for price prediction
- **Reinforcement Learning** - DQN, Policy Gradient, DDPG agents
- **Ensemble Learning** - Combine multiple models for robust predictions
- **AI Brain** - Real-time market analysis with caching

### 📊 Multi-Asset Trading (15+ Exchanges)
| Asset Class | Exchanges | Status |
|-------------|-----------|--------|
| **Crypto CEX** | Binance, Bybit, OKX, Coinbase, Kraken | ✅ Active |
| **Crypto DEX** | Uniswap, PancakeSwap | ✅ Active |
| **Perpetuals** | dYdX, Hyperliquid, Bitmex, Deribit | ✅ Active |
| **Stocks** | Alpaca, Interactive Brokers, Tradier | ✅ Active |
| **Forex** | Forex.com, OANDA | ✅ Active |
| **Options** | Tradier, Interactive Brokers | ✅ Active |

### 📈 Trading Strategies (15+)
- 🔲 **Grid Trading** - Automated grid orders for range-bound markets
- 💰 **DCA (Dollar Cost Averaging)** - Scheduled periodic purchases
- 📈 **Trend Following** - Moving average crossovers
- ⚡ **Scalping** - Quick high-frequency trades
- 🏪 **Market Making** - Bid-ask spread capture
- 🔄 **Mean Reversion** - Price deviation strategies
- 💥 **Breakout Trading** - Support/resistance breakout detection
- 🌀 **Momentum Trading** - RSI + MACD combination
- 🔗 **Pairs Trading** - Statistical arbitrage
- ⚖️ **Arbitrage** - Cross-exchange price differences
- ⏱️ **TWAP/VWAP** - Time/Volume-Weighted Average Price
- 🔒 **Bracket Orders** - Entry + Stop Loss + Take Profit
- 🦅 **Trailing Stop** - Dynamic stop loss following price
- 📊 **Hot Reload** - Real-time strategy reloading

### 📉 Technical Indicators (30+)
SMA, EMA, RSI, MACD, Bollinger Bands, ATR, Stochastic, ADX, OBV, CCI, VWAP, Ichimoku, Fibonacci, Pivot Points, SuperTrend, Keltner Channels, Alligator, MFI, WPR, CMF, STOCHRSI, Parabolic SAR, Aroon, TRIX, Momentum, ROC, and more...

### 🔧 System Features
- **CLI-Only Interface** - No UI/TUI, Linux-optimized
- **Full Backtest Engine** - Historical strategy validation with Sharpe, Sortino, Drawdown
- **Order Book Simulator** - Price heap-based simulation
- **Nanosecond Precision** - High-frequency trading support
- **Production Redis** - Full Redis client wrapper
- **Event Sourcing** - Event store for replay/debugging
- **Real Risk Manager** - Exposure, Drawdown, Daily Loss tracking
- **Performance Profiler** - Built-in benchmarking
- **REST API** - Full API documentation (OpenAPI 3.0)
- **WebSocket** - Real-time market data streaming
- **gRPC API** - High-performance trading API

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         OpenTrader                              │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   CLI      │  │  REST API   │  │     gRPC API            │ │
│  └──────┬──────┘  └──────┬──────┘  └───────────┬─────────────┘ │
│         │                │                      │                │
│  ┌──────▼────────────────▼──────────────────────▼─────────────┐ │
│  │                    Trading Engine                          │ │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────────────┐ │ │
│  │  │ Orders  │ │Portfolio│ │  Risk   │ │    Strategies   │ │ │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────────────┘ │ │
│  │  ┌─────────────────────────────────────────────────────┐ │ │
│  │  │               Backtest Engine                        │ │ │
│  │  │    (Sharpe, Sortino, Drawdown, P&L Tracking)        │ │ │
│  │  └─────────────────────────────────────────────────────┘ │ │
│  └────────────────────────┬────────────────────────────────────┘ │
│                           │                                     │
│  ┌────────────────────────▼────────────────────────────────────┐│
│  │                     AI Brain                                 ││
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────────────────┐  ││
│  │  │  LLM   │ │   ML   │ │   RL   │ │  Signal Generator  │  ││
│  │  └────────┘ └────────┘ └────────┘ └────────────────────┘  ││
│  └────────────────────────┬────────────────────────────────────┘│
│                           │                                     │
│  ┌────────────────────────▼────────────────────────────────────┐│
│  │              Exchange Adapters (15+)                         ││
│  │  ┌──────┐ ┌───────┐ ┌────────┐ ┌──────┐ ┌──────────┐ ││
│  │  │Binance│ │ Bybit │ │ Kraken │ │ dYdX │ │Hyperliquid│ ││
│  │  └──────┘ └───────┘ └────────┘ └──────┘ └──────────┘ ││
│  │  ┌──────────┐ ┌────────────┐ ┌─────────────────┐        ││
│  │  │Uniswap   │ │ Interactive │ │    Deribit      │        ││
│  │  │          │ │ Brokers     │ │                 │        ││
│  │  └──────────┘ └────────────┘ └─────────────────┘        ││
│  └───────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

## 🚀 Quick Start

### Prerequisites
- Go 1.22+
- Docker & Docker Compose (for deployment)
- PostgreSQL 16+ (optional, SQLite default)
- Redis 7+ (optional)

### Installation

```bash
# Clone the repository
git clone https://github.com/Nirmal09809/opentreder.git
cd opentreder

# Install dependencies
go mod download

# Build the application
go build -o opentreder ./cmd/cli

# Run the application
./opentreder
```

### Using Docker

```bash
# Start with Docker Compose
docker-compose up -d

# View logs
docker-compose logs -f opentreder
```

---

## 📖 Documentation

| Document | Description |
|----------|-------------|
| [Architecture](docs/architecture/architecture.md) | System architecture guide |
| [API Reference](docs/api/openapi.yaml) | REST & gRPC API documentation |
| [Strategy Development](docs/development/strategy-development.md) | Custom strategy creation |
| [Configuration](docs/configuration.md) | Full configuration reference |

---

## 💻 CLI Commands

```bash
# Start trading engine
./opentreder start

# Run backtest with full report
./opentreder backtest --strategy grid --symbol BTC/USDT

# Run backtest with optimizer
./opentreder backtest --strategy trend --symbol ETH/USDT --optimize

# List positions
./opentreder positions --exchange binance

# Place order
./opentreder order --symbol BTC/USDT --side buy --quantity 0.1 --type limit --price 45000

# Analyze market
./opentreder analyze --symbol ETH/USDT --timeframe 1h

# Run strategy
./opentreder run --strategy scalper --symbol BTC/USDT
```

---

## ⚙️ Configuration

Create `configs/config.yaml`:

```yaml
app:
  name: opentreder
  environment: production
  log_level: info
  data_dir: /tmp/opentreder

server:
  rest:
    host: 0.0.0.0
    port: 8080
  websocket:
    host: 0.0.0.0
    port: 8081
  grpc:
    host: 0.0.0.0
    port: 8082

database:
  sqlite:
    enabled: true
    path: ./data/opentreder.db
  postgresql:
    enabled: false
    host: localhost
    port: 5432

redis:
  enabled: true
  host: localhost
  port: 6379

exchanges:
  binance:
    enabled: true
    api_key: your_api_key
    api_secret: your_api_secret
    testnet: true
  interactive_brokers:
    enabled: false
    host: 127.0.0.1
    port: 4002

ai:
  enabled: true
  model: ensemble
  llm:
    provider: openai
    model: gpt-4
    api_key: your_openai_key

risk:
  max_position_size: 0.1
  max_drawdown: 0.2
  max_daily_loss: 0.05
```

---

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run unit tests
go test ./tests/unit/...

# Run with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./pkg/types/precision/...
```

---

## 📊 Stats

- **134 files**
- **377 directories**
- **34,569 lines of Go code**
- **15+ exchange adapters**
- **15+ trading strategies**
- **30+ technical indicators**

---

## 🐳 Deployment

### Docker Compose (Development)

```bash
docker-compose up -d
```

### Kubernetes

```bash
kubectl create namespace opentreder
kubectl apply -f deploy/k8s/base/
```

---

## 🤝 Contributing

Contributions are welcome!

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- [NautilusTrader](https://github.com/nautechsystems/nautilus_trader) - Reference architecture
- [Binance](https://www.binance.com/) - Crypto exchange API
- [Bybit](https://www.bybit.com/) - Crypto derivatives API
- [Interactive Brokers](https://www.interactivebrokers.com/) - Multi-asset trading
- [shopspring/decimal](https://github.com/shopspring/decimal) - Decimal arithmetic

---

<div align="center">

**Built with ❤️ for traders, by traders**

[![Stargazers](https://img.shields.io/github/stars/Nirmal09809/opentreder?style=social)](https://github.com/Nirmal09809/opentreder/stargazers)
[![Forks](https://img.shields.io/github/forks/Nirmal09809/opentreder?style=social)](https://github.com/Nirmal09809/opentreder/network/members)

</div>
