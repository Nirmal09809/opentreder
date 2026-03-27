# OpenTrader Architecture

## Overview

OpenTrader is an enterprise-grade AI-powered algorithmic trading framework designed for high-frequency trading across multiple cryptocurrency exchanges. Built with Go, it provides a modular, scalable architecture supporting real-time trading, backtesting, and AI-driven decision making.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Client Layer                                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │  Web UI     │  │  CLI Tool   │  │  Mobile App │  │  Third-Party Apps   │ │
│  │  (React)    │  │  (Go CLI)    │  │  (Flutter)  │  │  (REST/WebSocket)   │ │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘ │
└─────────┼────────────────┼────────────────┼────────────────────┼────────────┘
          │                │                │                    │
          └────────────────┴────────────────┴────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            API Gateway Layer                                │
│  ┌─────────────────────────────────────────────────────────────────────────┐ │
│  │                        REST API Server (Gin)                           │ │
│  │   - JWT Authentication    - Rate Limiting    - Request Validation       │ │
│  │   - API Versioning        - Response Compression - OpenAPI Docs       │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│  ┌────────────────────────────┐  ┌────────────────────────────────────────┐ │
│  │   WebSocket Server         │  │   gRPC Server                          │ │
│  │   - Real-time Updates      │  │   - High Performance API                │ │
│  │   - Market Data Streams    │  │   - Bidirectional Streaming             │ │
│  │   - Order Updates          │  │   - Service Mesh Compatible             │ │
│  └────────────────────────────┘  └────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Service Layer                                       │
│  ┌────────────────┐ ┌────────────────┐ ┌────────────────┐ ┌─────────────┐ │
│  │ Trading Engine │ │ Order Manager  │ │ Portfolio Mgr   │ │ Risk Manager│ │
│  │ - Order Exec   │ │ - Order Routing│ │ - Positions     │ │ - Position  │ │
│  │ - State Mgmt   │ │ - Fill Logic   │ │ - Balances      │ │ - Exposure  │ │
│  │ - Event Loop   │ │ - Modification │ │ - P&L Tracking  │ │ - Limits    │ │
│  └────────────────┘ └────────────────┘ └────────────────┘ └─────────────┘ │
│  ┌────────────────┐ ┌────────────────┐ ┌────────────────┐ ┌─────────────┐ │
│  │  AI Brain      │ │ Strategy Mgr   │ │ Market Data    │ │ Backtest    │ │
│  │ - LLM Client   │ │ - Strategy     │ │ - Aggregator   │ │ - Simulator  │ │
│  │ - Sentiment    │ │ - Parameters   │ │ - Normalizer   │ │ - Analytics  │ │
│  │ - Pattern Rec  │ │ - OnTick()     │ │ - Timeframes   │ │ - Walk-fwd   │ │
│  └────────────────┘ └────────────────┘ └────────────────┘ └─────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                       Exchange Adapter Layer                                │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌──────────────┐ │
│  │  Binance  │ │   Bybit   │ │ Coinbase  │ │  Alpaca   │ │   More...    │ │
│  └─────┬─────┘ └─────┬─────┘ └─────┬─────┘ └─────┬─────┘ └──────┬───────┘ │
│        └───────────────┴───────────────┴───────────────┴──────────────┘        │
│                                    │                                         │
│                          ┌─────────┴──────────┐                             │
│                          │  Exchange Interface │                             │
│                          │  - REST Client      │                             │
│                          │  - WebSocket Client│                             │
│                          │  - Signature Auth  │                             │
│                          │  - Rate Limiter    │                             │
│                          └────────────────────┘                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Data Layer                                           │
│  ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐            │
│  │    PostgreSQL    │ │      Redis       │ │    InfluxDB      │            │
│  │  - Orders        │ │  - Sessions      │ │  - Time-series   │            │
│  │  - Trades        │ │  - Cache        │ │  - Metrics       │            │
│  │  - Portfolios    │ │  - Rate Limit   │ │  - OHLCV Data   │            │
│  │  - Strategies    │ │  - Pub/Sub      │ │                  │            │
│  └──────────────────┘ └──────────────────┘ └──────────────────┘            │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Trading Engine

The trading engine is the heart of the system, managing the lifecycle of orders and maintaining state.

**Key Responsibilities:**
- Order execution and state management
- Position tracking and updates
- Event-driven architecture
- Strategy orchestration

```go
// Core engine interface
type Engine interface {
    Start(ctx context.Context) error
    Stop() error
    SubmitOrder(order *Order) error
    CancelOrder(orderID string) error
    GetPositions() []*Position
    GetOrders() []*Order
}
```

### 2. Order Manager

Handles order routing, modification, and tracking across exchanges.

**Features:**
- Smart order routing
- Order validation
- Fill simulation
- Order lifecycle management

### 3. Risk Manager

Real-time risk monitoring and enforcement.

**Risk Controls:**
- Position size limits
- Daily loss limits
- Leverage constraints
- Drawdown protection
- Correlation-based risk

### 4. AI Brain

Machine learning and LLM-powered trading insights.

**Components:**
- LLM Client (OpenAI, Anthropic, etc.)
- Sentiment Analyzer
- Pattern Recognition
- Reinforcement Learning Agent

### 5. Strategy Manager

Manages trading strategies with lifecycle control.

**Strategy Lifecycle:**
```
Created → Initialized → Running → Paused → Stopped
                        ↓
                    Error → Failed
```

## Data Flow

### Order Execution Flow

```
Client Request
      │
      ▼
┌─────────────┐
│ API Gateway │
│ - Auth      │
│ - Validate  │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│Risk Manager │
│ - Check     │
│ - Approve   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│Order Manager│
│ - Route     │
│ - Submit    │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Exchange    │
│ Adapter     │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  Exchange   │
│  (Binance) │
└──────┬──────┘
       │
       ▼
   Fill/Update
       │
       ▼
┌─────────────┐
│ Portfolio   │
│ Manager     │
└─────────────┘
```

### Market Data Flow

```
Exchange WebSocket
      │
      ▼
┌─────────────┐
│ Exchange    │
│ Adapter     │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Market Data│
│ Aggregator │
└──────┬──────┘
       │
       ├──▶ Strategies
       │
       ├──▶ UI Updates
       │
       └──▶ Storage (InfluxDB)
```

## Directory Structure

```
opentreder/
├── cmd/
│   └── cli/              # CLI application
│       └── main.go
│
├── internal/
│   ├── api/              # API layer
│   │   ├── grpc/         # gRPC service
│   │   ├── rest/         # REST API
│   │   └── websocket/    # WebSocket handler
│   │
│   ├── core/             # Core trading logic
│   │   ├── engine/       # Trading engine
│   │   ├── orders/       # Order management
│   │   ├── portfolio/    # Portfolio management
│   │   ├── positions/    # Position tracking
│   │   └── risk/         # Risk management
│   │
│   ├── exchanges/        # Exchange adapters
│   │   ├── binance/
│   │   ├── bybit/
│   │   ├── coinbase/
│   │   └── interface.go
│   │
│   ├── strategies/       # Trading strategies
│   │   ├── grid/
│   │   ├── dca/
│   │   ├── trend/
│   │   └── interface.go
│   │
│   ├── ai/               # AI components
│   │   ├── brain/        # Main AI brain
│   │   ├── ml/           # ML models
│   │   └── sentiment/     # Sentiment analysis
│   │
│   ├── marketdata/       # Market data handling
│   ├── storage/          # Data persistence
│   ├── backtest/         # Backtesting engine
│   └── ui/              # Terminal UI
│
├── pkg/
│   ├── types/           # Shared types
│   ├── config/          # Configuration
│   ├── logger/          # Logging
│   └── utils/           # Utilities
│
├── deploy/
│   ├── k8s/            # Kubernetes manifests
│   └── helm/           # Helm charts
│
├── docs/
│   ├── api/            # API documentation
│   ├── architecture/   # This file
│   └── development/   # Dev guides
│
└── tests/
    ├── unit/           # Unit tests
    ├── integration/    # Integration tests
    └── e2e/           # End-to-end tests
```

## Technology Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| Core Language | Go 1.24+ | Primary development language |
| REST API | Gin | HTTP framework |
| gRPC | grpc-go | High-performance API |
| WebSocket | gorilla/websocket | Real-time communication |
| Database | PostgreSQL | Primary data store |
| Cache | Redis | Session, cache, pub/sub |
| Time-series | InfluxDB | Market data storage |
| Container | Docker | Application packaging |
| Orchestration | Kubernetes | Deployment & scaling |
| Monitoring | Prometheus/Grafana | Metrics & visualization |

## Scalability

### Horizontal Scaling

The system is designed for horizontal scaling with:

1. **Stateless Services** - API layer is stateless, allowing multiple instances
2. **Distributed Caching** - Redis for shared state across instances
3. **Message Queue** - Redis Pub/Sub for event distribution
4. **Database Pooling** - Connection pooling for database efficiency

### Performance Targets

| Metric | Target |
|--------|--------|
| API Latency (p99) | < 100ms |
| Order Execution | < 50ms |
| WebSocket Throughput | 10,000 msg/sec |
| Concurrent Connections | 5,000+ |
| Backtest Speed | 1M candles/sec |

## Security

### Authentication Flow

```
1. Client → POST /auth/login {username, password}
2. Server → Validate credentials
3. Server → Generate JWT (access + refresh)
4. Client → Store tokens securely
5. Client → Include JWT in Authorization header
6. Server → Validate JWT on each request
```

### Security Measures

- JWT tokens with RS256 signing
- API key encryption at rest
- Rate limiting per endpoint
- Input validation and sanitization
- SQL injection prevention (parameterized queries)
- CORS configuration
- Security headers (HSTS, CSP, etc.)

## Monitoring

### Key Metrics

- **Business Metrics**
  - Total P&L
  - Win rate
  - Number of trades
  - Active positions

- **System Metrics**
  - API latency
  - Error rates
  - Memory/CPU usage
  - Database connections

- **Trading Metrics**
  - Order fill rate
  - Slippage
  - Position exposure
  - Risk utilization

## Deployment

### Kubernetes Deployment

```yaml
# High-level deployment structure
apiVersion: apps/v1
kind: Deployment
metadata:
  name: opentreder
spec:
  replicas: 3
  selector:
    matchLabels:
      app: opentreder
  template:
    spec:
      containers:
        - name: opentreder
          resources:
            requests:
              cpu: 500m
              memory: 512Mi
            limits:
              cpu: 2000m
              memory: 2Gi
```

### Helm Installation

```bash
# Install with custom values
helm install opentreder ./deploy/helm/opentreder \
  --set image.tag=v1.0.0 \
  --set replicaCount=3 \
  --set ingress.enabled=true
```

## Next Steps

- [API Documentation](../api/openapi.yaml)
- [Deployment Guide](./deployment.md)
- [Strategy Development](./strategy-development.md)
- [Configuration Reference](./configuration.md)
