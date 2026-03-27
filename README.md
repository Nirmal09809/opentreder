# OpenTrader 🚀

### Enterprise-Grade AI-Powered Trading Framework

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8.svg)](https://golang.org/)
[![Build Status](https://github.com/opentreder/opentreder/actions/workflows/ci.yml/badge.svg)](https://github.com/opentreder/opentreder/actions)
[![Code Coverage](https://codecov.io/gh/opentreder/opentreder/branch/main/graph/badge.svg)](https://codecov.io/gh/opentreder/opentreder)
[![Go Report Card](https://goreportcard.com/badge/github.com/opentreder/opentreder)](https://goreportcard.com/report/github.com/opentreder/opentreder)
[![Discord](https://img.shields.io/badge/Discord-Join-7289DA.svg?logo=discord)](https://discord.gg/opentreder)

---

## ✨ Features

### 🤖 AI-Powered Trading
- **LLM Integration** - GPT-4 powered market analysis and decision making
- **Machine Learning Models** - XGBoost, LSTM, Transformer for price prediction
- **Reinforcement Learning** - DQN, Policy Gradient, DDPG agents
- **Ensemble Learning** - Combine multiple models for robust predictions

### 📊 Multi-Asset Trading
| Asset Class | Exchanges | Status |
|-------------|-----------|--------|
| **Crypto** | Binance, Bybit, Coinbase | ✅ Active |
| **Stocks** | Alpaca, Tradier | ✅ Active |
| **Forex** | Forex.com | ✅ Active |
| **Options** | Tradier | ✅ Active |

### 📈 Trading Strategies
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
- ⏱️ **TWAP** - Time-Weighted Average Price execution
- 📊 **VWAP** - Volume-Weighted Average Price execution
- 🔒 **Bracket Orders** - Entry + Stop Loss + Take Profit
- 🦅 **Trailing Stop** - Dynamic stop loss following price

### 📉 Technical Indicators (30+)
SMA, EMA, RSI, MACD, Bollinger Bands, ATR, Stochastic, ADX, OBV, CCI, VWAP, Ichimoku, Fibonacci, Pivot Points, SuperTrend, Keltner Channels, Alligator, MFI, WPR, CMF, STOCHRSI, Parabolic SAR, Aroon, TRIX, Momentum, ROC, and more...

### 🔧 System Features
- **Terminal UI** - Beautiful Lipgloss-based TUI
- **Web Dashboard** - Real-time trading visualization
- **REST API** - Full API documentation (OpenAPI 3.0)
- **WebSocket** - Real-time market data streaming
- **gRPC API** - High-performance trading API
- **Backtesting** - Historical strategy validation
- **Optimizer** - Genetic, Grid, Bayesian optimization

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         OpenTrader                               │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   CLI/TUI   │  │  Web UI     │  │     REST/gRPC API       │ │
│  └──────┬──────┘  └──────┬──────┘  └───────────┬─────────────┘ │
│         │                │                      │                │
│  ┌──────▼────────────────▼──────────────────────▼─────────────┐ │
│  │                    Trading Engine                          │ │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────────────┐ │ │
│  │  │ Orders  │ │ Portfolio│ │  Risk   │ │    Strategies   │ │ │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────────────┘ │ │
│  └────────────────────────┬────────────────────────────────────┘ │
│                           │                                     │
│  ┌────────────────────────▼────────────────────────────────────┐│
│  │                     AI Brain                                ││
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────────────────┐  ││
│  │  │  LLM   │ │   ML   │ │   RL   │ │  Signal Generator  │  ││
│  │  └────────┘ └────────┘ └────────┘ └────────────────────┘  ││
│  └────────────────────────┬────────────────────────────────────┘│
│                           │                                     │
│  ┌────────────────────────▼────────────────────────────────────┐│
│  │              Exchange Adapters                              ││
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────┐ ││
│  │  │ Binance │ │  Bybit  │ │Coinbase │ │ Alpaca  │ │ ... │ ││
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────┘ ││
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
git clone https://github.com/opentreder/opentreder.git
cd opentreder

# Install dependencies & setup
make setup

# Build the application
make build

# Run the application
make run
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
| [API Reference](docs/api/openapi.yaml) | REST & gRPC API documentation (OpenAPI 3.0) |
| [Deployment Guide](docs/architecture/deployment.md) | Kubernetes & Helm deployment |
| [Strategy Development](docs/development/strategy-development.md) | Custom strategy creation |
| [Configuration](docs/configuration.md) | Full configuration reference |
| [Contributing](CONTRIBUTING.md) | Contribution guidelines |

---

## 💻 Usage

### CLI Commands

```bash
# Start trading engine
./opentreder start

# Run backtest
./opentreder backtest --strategy grid --symbol BTC/USDT

# Analyze market
./opentreder analyze --symbol ETH/USDT --timeframe 1h

# List positions
./opentreder positions --exchange binance

# Place order
./opentreder order --symbol BTC/USDT --side buy --quantity 0.1 --type limit --price 45000
```

### API Examples

```bash
# Get account
curl http://localhost:8080/v1/account?exchange=binance

# Place order
curl -X POST http://localhost:8080/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"symbol":"BTC/USDT","side":"buy","quantity":"0.01","type":"market"}'

# Get quotes
curl http://localhost:8080/v1/market/quote?symbol=BTC/USDT&exchange=binance
```

---

## ⚙️ Configuration

Create `configs/config.yaml`:

```yaml
app:
  name: opentreder
  env: production
  log_level: info

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
    path: ./data/opentreder.db
  postgres:
    enabled: false
    host: localhost
    port: 5432

redis:
  host: localhost
  port: 6379

exchanges:
  binance:
    enabled: true
    api_key: your_api_key
    api_secret: your_api_secret
    testnet: true

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
# Run unit tests
go test ./tests/unit/...

# Run integration tests
go test -tags=integration ./tests/integration/...

# Run all tests with coverage
make test-coverage

# Run benchmarks
make bench
```

---

## 🐳 Deployment

### Kubernetes

```bash
kubectl create namespace opentreder
kubectl apply -f deploy/k8s/base/
```

### Helm

```bash
helm install opentreder ./deploy/helm/opentreder \
  --namespace opentreder \
  --create-namespace
```

### Docker Compose (Development)

```bash
docker-compose up -d
```

---

## 📊 Monitoring

### Prometheus Metrics
Access at `http://localhost:8080/metrics`

### Grafana Dashboard
Included in Helm chart - auto-configured

### Key Metrics
- `opentreder_http_requests_total` - Total HTTP requests
- `opentreder_orders_total` - Total orders placed
- `opentreder_portfolio_value` - Current portfolio value
- `opentreder_portfolio_pnl` - Profit & Loss
- `opentreder_position_risk_ratio` - Position risk metrics

---

## 🤝 Contributing

Contributions are welcome! Please read our [Contributing Guidelines](CONTRIBUTING.md) before submitting PRs.

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

- [Binance](https://www.binance.com/) - Crypto exchange API
- [Bybit](https://www.bybit.com/) - Crypto derivatives API
- [Alpaca](https://alpaca.markets/) - Stock trading API
- [Tradier](https://tradier.com/) - Options trading API
- [shopspring/decimal](https://github.com/shopspring/decimal) - Decimal arithmetic

---

<div align="center">

**Built with ❤️ for traders, by traders**

[![Stargazers](https://img.shields.io/github/stars/opentreder/opentreder?style=social)](https://github.com/opentreder/opentreder/stargazers)
[![Forks](https://img.shields.io/github/forks/opentreder/opentreder?style=social)](https://github.com/opentreder/opentreder/network/members)
[![Watchers](https://img.shields.io/github/watchers/opentreder/opentreder?style=social)](https://github.com/opentreder/opentreder/watchers)

</div>
