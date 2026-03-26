# Contributing to OpenTrader

Thank you for your interest in contributing to OpenTrader! 🚀

This document provides guidelines and instructions for contributing to this project.

---

## 📋 Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Commit Guidelines](#commit-guidelines)
- [Pull Request Process](#pull-request-process)
- [Testing](#testing)
- [Documentation](#documentation)

---

## 📜 Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for everyone. We expect all contributors to:

- Be respectful and considerate in communication
- Accept constructive criticism gracefully
- Focus on what is best for the community
- Show empathy towards other community members

---

## 🏃 Getting Started

1. **Fork the Repository**
   ```bash
   git clone https://github.com/YOUR_USERNAME/opentreder.git
   cd opentreder
   ```

2. **Add Upstream Remote**
   ```bash
   git remote add upstream https://github.com/opentreder/opentreder.git
   ```

3. **Sync with Upstream**
   ```bash
   git fetch upstream
   git checkout main
   git merge upstream/main
   ```

---

## 🔧 Development Setup

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- Make

### Setup Development Environment

```bash
# Install dependencies
make setup

# Run development server
make run

# Run tests
make test

# Run linter
make lint
```

### Environment Variables

Create a `.env` file for local development:

```env
# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=opentreder
POSTGRES_PASSWORD=your_password
POSTGRES_DB=opentreder

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# Exchange API Keys (use testnet for development)
BINANCE_API_KEY=your_key
BINANCE_API_SECRET=your_secret
BYBIT_API_KEY=your_key
BYBIT_API_SECRET=your_secret

# AI/LLM
OPENAI_API_KEY=your_key
```

---

## 🔨 Making Changes

### 1. Create a Feature Branch

```bash
# Create a new branch from main
git checkout -b feature/your-feature-name

# Or for bug fixes
git checkout -b fix/your-bug-fix
```

### 2. Branch Naming Conventions

| Type | Pattern | Example |
|------|---------|---------|
| Feature | `feature/` | `feature/add-strategy-optimizer` |
| Bug Fix | `fix/` | `fix/order-execution-delay` |
| Documentation | `docs/` | `docs/api-reference` |
| Refactor | `refactor/` | `refactor/cleanup-engine` |
| Test | `test/` | `test/add-backtest-tests` |
| Performance | `perf/` | `perf/improve-order-latency` |

### 3. Write Code

Follow these coding standards:

- **Go Style Guide**: Follow [Effective Go](https://go.dev/doc/effective_go)
- **Formatting**: Use `gofmt` and `goimports`
- **Naming**: Use descriptive names (`GetOrderByID`, not `GetO`)
- **Error Handling**: Always handle errors explicitly
- **Comments**: Document exported functions and types

### 4. Code Structure

```
internal/
├── core/           # Core trading logic
│   ├── engine/     # Trading engine
│   ├── orders/     # Order management
│   ├── risk/       # Risk management
│   └── portfolio/  # Portfolio management
├── exchanges/      # Exchange adapters
│   ├── binance/
│   ├── bybit/
│   └── ...
├── ai/             # AI/ML components
│   ├── brain.go
│   └── ml/
├── strategies/     # Trading strategies
├── api/            # API servers
│   ├── rest/
│   ├── websocket/
│   └── grpc/
└── ui/             # User interfaces
    ├── tui/
    └── dashboard/
```

---

## 📝 Commit Guidelines

### Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation changes |
| `style` | Code style changes (formatting, etc.) |
| `refactor` | Code refactoring |
| `perf` | Performance improvements |
| `test` | Adding tests |
| `chore` | Maintenance tasks |

### Examples

```bash
# Feature
git commit -m "feat(exchange): add Binance futures support"

# Bug fix
git commit -m "fix(orders): resolve race condition in order queue"

# Documentation
git commit -m "docs(api): update REST API documentation"

# Refactor
git commit -m "refactor(engine): improve order execution pipeline"
```

---

## 🔀 Pull Request Process

### 1. Before Submitting

- [ ] Code follows project style guidelines
- [ ] Tests pass (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated if needed
- [ ] Commit messages are clear and descriptive

### 2. Creating Pull Request

1. Push your branch:
   ```bash
   git push origin feature/your-feature-name
   ```

2. Open a Pull Request on GitHub

3. Fill out the PR template:
   ```markdown
   ## Description
   Brief description of changes

   ## Type of Change
   - [ ] Bug fix
   - [ ] New feature
   - [ ] Breaking change
   - [ ] Documentation update

   ## Testing
   Describe tests you ran

   ## Checklist
   - [ ] My code follows the style guidelines
   - [ ] I have performed a self-review
   - [ ] I have commented my code where needed
   - [ ] I have updated documentation
   ```

### 3. Review Process

- PRs require at least one approval
- All checks must pass
- Address any feedback promptly

---

## 🧪 Testing

### Running Tests

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific package tests
go test ./internal/core/...

# Run benchmarks
make bench
```

### Writing Tests

```go
func TestOrderCreation(t *testing.T) {
    order := &Order{
        ID:     GenerateUUID(),
        Symbol: "BTC/USDT",
        Side:   SideBuy,
    }
    
    assert.NotEmpty(t, order.ID)
    assert.Equal(t, "BTC/USDT", order.Symbol)
}
```

### Test Coverage Goals

| Package | Target |
|--------|--------|
| Core | 80%+ |
| Exchanges | 70%+ |
| Strategies | 70%+ |
| API | 60%+ |

---

## 📚 Documentation

### Where to Document

| Type | Location |
|------|----------|
| Code comments | In source files |
| API docs | `docs/api.md` |
| Configuration | `docs/configuration.md` |
| Strategy guide | `docs/strategies.md` |

### Documentation Style

- Use clear, concise language
- Include code examples where possible
- Add diagrams for complex systems
- Keep documentation up-to-date

---

## 🆘 Getting Help

- **GitHub Issues**: [Open an issue](https://github.com/opentreder/opentreder/issues)
- **Discord**: [Join our server](https://discord.gg/opentreder)
- **Documentation**: [docs/](docs/)

---

## 📄 License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

---

<div align="center">

**Thank you for contributing to OpenTrader!** 🎉

</div>
