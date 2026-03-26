# Deployment Guide

## Prerequisites

- Docker & Docker Compose
- Kubernetes cluster (optional, for K8s deployment)
- PostgreSQL 16+
- Redis 7+
- Go 1.22+

## Local Development

### Quick Start

```bash
# Clone the repository
git clone https://github.com/opentreder/opentreder.git
cd opentreder

# Install dependencies
make setup

# Build the application
make build

# Run the application
make run
```

### Using Docker Compose

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f opentreder

# Stop services
docker-compose down
```

## Production Deployment

### Docker

```bash
# Build the image
docker build -t opentreder:latest .

# Run with environment variables
docker run -d \
  --name opentreder \
  -p 8080:8080 \
  -p 8081:8081 \
  -p 8082:8082 \
  -p 9090:9090 \
  -e POSTGRES_HOST=postgres \
  -e REDIS_HOST=redis \
  -e BINANCE_API_KEY=your_key \
  -e BINANCE_API_SECRET=your_secret \
  -v opentreder_data:/data \
  opentreder:latest
```

### Kubernetes

```bash
# Create namespace
kubectl create namespace opentreder

# Apply configurations
kubectl apply -f deploy/kubernetes/

# Check deployment status
kubectl get pods -n opentreder

# View logs
kubectl logs -n opentreder -l app=opentreder
```

### Helm Chart

```bash
# Add Helm repo (if published)
helm repo add opentreder https://charts.opentreder.io

# Install with Helm
helm install opentreder opentreder/opentreder \
  --namespace opentreder \
  --create-namespace \
  --set image.tag=v1.0.0 \
  --set environment=production
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `POSTGRES_HOST` | PostgreSQL host | localhost |
| `POSTGRES_PORT` | PostgreSQL port | 5432 |
| `POSTGRES_USER` | PostgreSQL user | opentreder |
| `POSTGRES_PASSWORD` | PostgreSQL password | - |
| `POSTGRES_DB` | Database name | opentreder |
| `REDIS_HOST` | Redis host | localhost |
| `REDIS_PORT` | Redis port | 6379 |
| `REDIS_PASSWORD` | Redis password | - |
| `BINANCE_API_KEY` | Binance API key | - |
| `BINANCE_API_SECRET` | Binance API secret | - |
| `BYBIT_API_KEY` | Bybit API key | - |
| `BYBIT_API_SECRET` | Bybit API secret | - |
| `OT_LOG_LEVEL` | Log level | info |

### Configuration File

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
    path: /data/opentreder.db
  postgres:
    enabled: true
    host: localhost
    port: 5432
    user: opentreder
    password: password
    database: opentreder

redis:
  host: localhost
  port: 6379
  password: ""

exchanges:
  binance:
    enabled: true
    api_key: your_api_key
    api_secret: your_api_secret
    testnet: false
  bybit:
    enabled: true
    api_key: your_api_key
    api_secret: your_api_secret
    testnet: false

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
  max_leverage: 10
```

## Monitoring

### Prometheus Metrics

Access metrics at `http://localhost:9090/metrics`

Key metrics:
- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request latency histogram
- `orders_total` - Total orders placed
- `orders_failed_total` - Failed orders
- `portfolio_value` - Current portfolio value
- `portfolio_pnl` - Unrealized PnL

### Grafana Dashboards

Import `deploy/monitoring/grafana_dashboard.json` into Grafana.

### Alerting

Prometheus alerting rules are in `deploy/monitoring/alert_rules.yml`.

## API Documentation

Swagger UI: `http://localhost:8080/swagger/`

OpenAPI spec: `http://localhost:8080/openapi.yaml`

## Security

### API Keys

- Never commit API keys to version control
- Use Kubernetes secrets for production
- Rotate keys regularly

### Network

- Use TLS in production
- Restrict access to management ports
- Implement rate limiting

## Troubleshooting

### Logs

```bash
# View application logs
kubectl logs -n opentreder -l app=opentreder

# View with tail
kubectl logs -n opentreder -l app=opentreder -f
```

### Database Connection

```bash
# Test database connection
kubectl exec -it -n opentreder deployment/opentreder -- nc -zv postgres-svc 5432
```

### Redis Connection

```bash
# Test Redis connection
kubectl exec -it -n opentreder deployment/opentreder -- redis-cli -h redis-svc ping
```

## Backup & Recovery

### Database Backup

```bash
# Backup PostgreSQL
pg_dump -h localhost -U opentreder -d opentreder > backup.sql

# Restore
psql -h localhost -U opentreder -d opentreder < backup.sql
```

### Volume Backup

```bash
# Create snapshot
kubectl create -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: opentreder-snapshot
  namespace: opentreder
spec:
  volumeSnapshotClassName: csi-aws-volsnap
  source:
    persistentVolumeClaimName: opentreder-data
EOF
```

## Scaling

### Horizontal Pod Autoscaling

The K8s deployment includes HPA configuration for automatic scaling:

```bash
# View HPA status
kubectl get hpa -n opentreder

# Manually scale
kubectl scale deployment opentreder --replicas=5 -n opentreder
```

## Support

- GitHub Issues: https://github.com/opentreder/opentreder/issues
- Documentation: https://docs.opentreder.io
- Discord: https://discord.gg/opentreder
