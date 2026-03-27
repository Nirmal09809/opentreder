# OpenTrader Deployment Guide

## Prerequisites

- Kubernetes 1.24+
- Helm 3.10+
- PostgreSQL 16+ (or managed service)
- Redis 7+
- Docker (for local development)

## Deployment Options

### 1. Kubernetes Deployment (Production)

#### Using Helm

```bash
# Add Helm repository (if published)
helm repo add opentreder https://charts.opentreder.example.com
helm repo update

# Install with default values
helm install opentreder opentreder/opentreder

# Or install with custom values file
helm install opentreder ./deploy/helm/opentreder \
  --values ./deploy/helm/opentreder/values.yaml
```

#### Manual Kubernetes Manifests

```bash
# Apply all manifests
kubectl apply -f deploy/k8s/base/

# Verify deployment
kubectl get pods -n opentreder
```

### 2. Docker Compose (Development/Testing)

```yaml
version: '3.8'

services:
  opentreder:
    image: ghcr.io/nirmal09809/opentreder:latest
    ports:
      - "8080:8080"
      - "8081:8081"
      - "9090:9090"
    environment:
      - OPENTRADER_ENV=development
    volumes:
      - ./configs:/config
      - ./data:/data
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: opentreder
      POSTGRES_USER: opentreder
      POSTGRES_PASSWORD: password
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass password
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

```bash
# Start services
docker-compose up -d

# View logs
docker-compose logs -f opentreder

# Stop services
docker-compose down
```

### 3. Local Development

```bash
# Clone repository
git clone https://github.com/Nirmal09809/opentreder.git
cd opentreder

# Install dependencies
go mod download

# Build binary
go build -o opentreder ./cmd/cli

# Run with default config
./opentreder run

# Or with custom config
./opentreder run --config /path/to/config.yaml
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENTRADER_ENV` | Environment mode | `development` |
| `OPENTRADER_CONFIG_PATH` | Config file path | `./configs/config.yaml` |
| `GIN_MODE` | Gin framework mode | `debug` |

### Configuration File

Create `configs/config.yaml`:

```yaml
app:
  name: opentreder
  version: 1.0.0
  environment: production
  debug: false
  data_dir: /data
  config_dir: /config
  log_dir: /logs

database:
  mode: postgresql
  postgresql:
    enabled: true
    host: localhost
    port: 5432
    database: opentreder
    user: opentreder
    password: your_password
    ssl_mode: disable
  max_open_conns: 25
  max_idle_conns: 5

cache:
  enabled: true
  redis:
    host: localhost
    port: 6379
    password: your_redis_password
  ttl: 5m

trading:
  mode: paper  # paper, live, backtest
  initial_balance: 100000
  max_positions: 10
  default_timeframe: 1h

risk:
  max_position_size: 0.1
  max_daily_loss: 0.05
  max_leverage: 3
  max_drawdown: 0.2

api:
  rest:
    enabled: true
    host: 0.0.0.0
    port: 8080
    rate_limit: 100
  websocket:
    enabled: true
    host: 0.0.0.0
    port: 8081
  grpc:
    enabled: true
    host: 0.0.0.0
    port: 9090

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
  provider: openai
  model: gpt-4
  api_key: your_openai_key

logging:
  level: info
  format: json
  output: /logs/opentreder.log
```

## Kubernetes Setup

### 1. Create Namespace

```bash
kubectl create namespace opentreder
```

### 2. Apply Base Manifests

```bash
# Apply all base configurations
kubectl apply -f deploy/k8s/base/

# Check status
kubectl get all -n opentreder
```

### 3. Configure Secrets

```bash
# Edit secrets with your actual values
kubectl edit secret opentreder-secrets -n opentreder

# Or create from file
kubectl create secret generic opentreder-secrets \
  --from-literal=binance-api-key=YOUR_KEY \
  --from-literal=binance-api-secret=YOUR_SECRET \
  --from-literal=openai-api-key=YOUR_KEY \
  --from-literal=postgres-password=YOUR_PASSWORD \
  -n opentreder
```

### 4. Verify Deployment

```bash
# Check pods
kubectl get pods -n opentreder

# Check services
kubectl get svc -n opentreder

# View logs
kubectl logs -n opentreder -l app=opentreder --tail=100

# Check pod details
kubectl describe pod -n opentreder -l app=opentreder
```

## Monitoring Setup

### Prometheus

Prometheus metrics are exposed at `/metrics`:

```yaml
# Example Prometheus scrape config
scrape_configs:
  - job_name: 'opentreder'
    static_configs:
      - targets: ['opentreder-service:8080']
    metrics_path: /metrics
```

### Grafana

Import the dashboard from `deploy/helm/opentreder/templates/monitoring.yaml`:

```bash
# Port forward to access Grafana
kubectl port-forward -n monitoring svc/grafana 3000:3000

# Access at http://localhost:3000
```

### Alerting

Alerts are configured in the PrometheusRule. To view:

```bash
kubectl get prometheusrule -n opentreder
kubectl describe prometheusrule opentreder-alerts -n opentreder
```

## Scaling

### Horizontal Pod Autoscaler

The HPA is already configured:

```bash
# View HPA status
kubectl get hpa -n opentreder

# Manual scale
kubectl scale deployment opentreder --replicas=5 -n opentreder
```

### Resource Tuning

Adjust in `values.yaml`:

```yaml
resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 2000m
    memory: 2Gi
```

## Troubleshooting

### Pod Not Starting

```bash
# Check pod events
kubectl describe pod <pod-name> -n opentreder

# View logs
kubectl logs <pod-name> -n opentreder --previous
```

### Database Connection Issues

```bash
# Check if postgres is running
kubectl get pods -n opentreder | grep postgres

# Test connection
kubectl run -it --rm debug \
  --image=postgres:16-alpine \
  --restart=Never \
  -n opentreder \
  -- psql -h postgres-service -U opentreder -d opentreder
```

### High Memory Usage

```bash
# Check resource usage
kubectl top pods -n opentreder

# Increase memory limit in values.yaml
resources:
  limits:
    memory: 4Gi
```

## Backup & Recovery

### Database Backup

```bash
# Create backup
kubectl exec -n opentreder deployment/postgresql -- \
  pg_dump -U opentreder opentreder > backup.sql

# Restore backup
kubectl exec -i -n opentreder deployment/postgresql -- \
  psql -U opentreder opentreder < backup.sql
```

### Volume Snapshots

```bash
# Create snapshot
kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: opentreder-backup
  namespace: opentreder
spec:
  volumeSnapshotClassName: csi-aws-vsc
  source:
    persistentVolumeClaimName: opentreder-data
EOF
```

## Security

### Network Policies

Network policies restrict traffic:

```bash
# View network policies
kubectl get networkpolicy -n opentreder
```

### Pod Security

Pods run as non-root with read-only filesystem by default.

### Secrets Management

For production, consider:
- HashiCorp Vault
- AWS Secrets Manager
- Azure Key Vault

## Upgrade

### Helm Upgrade

```bash
# Update Helm repo
helm repo update

# Upgrade release
helm upgrade opentreder ./deploy/helm/opentreder \
  --values ./deploy/helm/opentreder/values.yaml

# Rollback if needed
helm rollback opentreder -n opentreder
```

### Rolling Update

```bash
# Update image
kubectl set image deployment/opentreder \
  opentreder=ghcr.io/nirmal09809/opentreder:v1.1.0 \
  -n opentreder

# Watch rollout
kubectl rollout status deployment/opentreder -n opentreder
```

## Support

- GitHub Issues: https://github.com/Nirmal09809/opentreder/issues
- Documentation: https://github.com/Nirmal09809/opentreder/docs
