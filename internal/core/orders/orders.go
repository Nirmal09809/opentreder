package orders

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opentreder/opentreder/internal/core/engine"
	"github.com/opentreder/opentreder/pkg/logger"
	"github.com/opentreder/opentreder/pkg/types"
)

type Manager struct {
	engine    *engine.Engine
	orders    map[uuid.UUID]*types.Order
	pending   map[uuid.UUID]*types.Order
	completed map[uuid.UUID]*types.Order
	cancelled map[uuid.UUID]*types.Order
	mu        sync.RWMutex
}

func NewManager(e *engine.Engine) *Manager {
	return &Manager{
		engine:    e,
		orders:    make(map[uuid.UUID]*types.Order),
		pending:   make(map[uuid.UUID]*types.Order),
		completed: make(map[uuid.UUID]*types.Order),
		cancelled: make(map[uuid.UUID]*types.Order),
	}
}

func (m *Manager) PlaceOrder(order *types.Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if order.ID == uuid.Nil {
		order.ID = uuid.New()
	}

	order.Status = types.OrderStatusPending
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	m.orders[order.ID] = order
	m.pending[order.ID] = order

	logger.Info("Order placed",
		"order_id", order.ID,
		"symbol", order.Symbol,
		"side", order.Side,
		"type", order.Type,
		"quantity", order.Quantity,
	)

	go m.executeOrder(order)

	return nil
}

func (m *Manager) executeOrder(order *types.Order) {
	m.mu.Lock()
	delete(m.pending, order.ID)
	order.Status = types.OrderStatusOpen
	order.UpdatedAt = time.Now()
	m.mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	m.mu.Lock()
	order.Status = types.OrderStatusFilled
	order.FilledQuantity = order.Quantity
	order.AvgFillPrice = order.Price
	now := time.Now()
	order.FilledAt = &now
	order.UpdatedAt = time.Now()
	m.completed[order.ID] = order
	m.mu.Unlock()

	logger.Info("Order filled",
		"order_id", order.ID,
		"symbol", order.Symbol,
		"quantity", order.FilledQuantity,
		"price", order.AvgFillPrice,
	)

	m.engine.Publish(engine.Event{
		Type:    engine.EventOrderFilled,
		Payload: order,
		Time:    time.Now(),
	})
}

func (m *Manager) CancelOrder(orderID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, exists := m.orders[orderID]
	if !exists {
		return fmt.Errorf("order not found: %s", orderID)
	}

	if order.Status == types.OrderStatusFilled {
		return fmt.Errorf("cannot cancel filled order")
	}

	if order.Status == types.OrderStatusCancelled {
		return fmt.Errorf("order already cancelled")
	}

	delete(m.pending, orderID)
	order.Status = types.OrderStatusCancelled
	now := time.Now()
	order.CancelledAt = &now
	order.UpdatedAt = time.Now()
	m.cancelled[order.ID] = order

	logger.Info("Order cancelled", "order_id", orderID)

	return nil
}

func (m *Manager) GetOrder(orderID uuid.UUID) *types.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.orders[orderID]
}

func (m *Manager) GetAllOrders() []*types.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orders := make([]*types.Order, 0, len(m.orders))
	for _, order := range m.orders {
		orders = append(orders, order)
	}
	return orders
}

func (m *Manager) GetPendingOrders() []*types.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()

	orders := make([]*types.Order, 0, len(m.pending))
	for _, order := range m.pending {
		orders = append(orders, order)
	}
	return orders
}

func (m *Manager) GetOpenOrders() []*types.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var orders []*types.Order
	for _, order := range m.orders {
		if order.Status == types.OrderStatusOpen || order.Status == types.OrderStatusPartiallyFilled {
			orders = append(orders, order)
		}
	}
	return orders
}

func (m *Manager) GetOrdersBySymbol(symbol string) []*types.Order {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var orders []*types.Order
	for _, order := range m.orders {
		if order.Symbol == symbol {
			orders = append(orders, order)
		}
	}
	return orders
}

func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total":     len(m.orders),
		"pending":   len(m.pending),
		"completed": len(m.completed),
		"cancelled": len(m.cancelled),
	}
}
