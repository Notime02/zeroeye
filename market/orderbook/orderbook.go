package orderbook

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/tent-of-trials/market/types"
)

type Config struct {
	MaxDepth       int
	PriceDecimals  int32
	VolumeDecimals int32
}

type OrderBook struct {
	mu        sync.RWMutex
	symbol    types.Symbol
	config    Config
	bids      []*types.Level
	asks      []*types.Level
	orders    map[string]*types.Order
	sequence  uint64
	updatedAt time.Time
	closed    bool
}

const snapshotVersion = 1

type snapshotData struct {
	Version   int           `json:"version"`
	Symbol    types.Symbol  `json:"symbol"`
	Config    Config        `json:"config"`
	Sequence  uint64        `json:"sequence"`
	UpdatedAt time.Time     `json:"updated_at"`
	Bids      []types.Level `json:"bids"`
	Asks      []types.Level `json:"asks"`
	Orders    []types.Order `json:"orders"`
}

func NewOrderBook(symbol types.Symbol, config Config) *OrderBook {
	return &OrderBook{
		symbol:   symbol,
		config:   config,
		bids:     make([]*types.Level, 0, config.MaxDepth),
		asks:     make([]*types.Level, 0, config.MaxDepth),
		orders:   make(map[string]*types.Order),
		sequence: 0,
	}
}

func (ob *OrderBook) Snapshot() ([]byte, error) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	snapshot := snapshotData{
		Version:   snapshotVersion,
		Symbol:    ob.symbol,
		Config:    ob.config,
		Sequence:  ob.sequence,
		UpdatedAt: ob.updatedAt,
		Bids:      copyLevelValues(ob.bids),
		Asks:      copyLevelValues(ob.asks),
		Orders:    copyOrderValues(ob.orders),
	}

	return json.Marshal(snapshot)
}

func (ob *OrderBook) Recover(data []byte) error {
	var snapshot snapshotData
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("decode order book snapshot: %w", err)
	}
	if snapshot.Version != snapshotVersion {
		return fmt.Errorf("unsupported order book snapshot version %d", snapshot.Version)
	}
	if snapshot.Symbol == "" {
		return fmt.Errorf("order book snapshot missing symbol")
	}
	if ob.symbol != "" && ob.symbol != snapshot.Symbol {
		return fmt.Errorf("order book snapshot symbol %q does not match %q", snapshot.Symbol, ob.symbol)
	}

	orders := make(map[string]*types.Order, len(snapshot.Orders))
	for i := range snapshot.Orders {
		order := snapshot.Orders[i]
		if order.ID == "" {
			return fmt.Errorf("order book snapshot contains order without id")
		}
		if order.Symbol != "" && order.Symbol != snapshot.Symbol {
			return fmt.Errorf("order %q symbol %q does not match snapshot symbol %q", order.ID, order.Symbol, snapshot.Symbol)
		}
		if _, exists := orders[order.ID]; exists {
			return fmt.Errorf("order book snapshot contains duplicate order id %q", order.ID)
		}
		orderCopy := order
		orders[order.ID] = &orderCopy
	}

	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.symbol = snapshot.Symbol
	ob.config = snapshot.Config
	ob.sequence = snapshot.Sequence
	ob.updatedAt = snapshot.UpdatedAt
	ob.bids = copyLevelPointers(snapshot.Bids)
	ob.asks = copyLevelPointers(snapshot.Asks)
	ob.orders = orders
	ob.closed = false
	return nil
}

func (ob *OrderBook) AddOrder(order *types.Order) ([]*types.Trade, error) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if ob.closed {
		return nil, ErrBookClosed
	}

	if order.ID == "" {
		order.ID = uuid.New().String()
	}

	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()
	order.Status = types.New

	ob.orders[order.ID] = order
	ob.sequence++

	level := &types.Level{
		Price:    order.Price,
		Quantity: order.RemainingQty,
		Count:    1,
	}

	if order.Side == types.Buy {
		ob.bids = insertLevel(ob.bids, level, true)
	} else {
		ob.asks = insertLevel(ob.asks, level, false)
	}

	ob.updatedAt = time.Now()
	return nil, nil
}

func (ob *OrderBook) CancelOrder(orderID string) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if ob.closed {
		return ErrBookClosed
	}

	order, exists := ob.orders[orderID]
	if !exists {
		return ErrOrderNotFound
	}

	order.Status = types.Cancelled
	order.UpdatedAt = time.Now()
	delete(ob.orders, orderID)

	if order.Side == types.Buy {
		ob.bids = removeLevel(ob.bids, order.Price)
	} else {
		ob.asks = removeLevel(ob.asks, order.Price)
	}

	ob.updatedAt = time.Now()
	return nil
}

func (ob *OrderBook) GetBids() []*types.Level {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	result := make([]*types.Level, len(ob.bids))
	copy(result, ob.bids)
	return result
}

func (ob *OrderBook) GetAsks() []*types.Level {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	result := make([]*types.Level, len(ob.asks))
	copy(result, ob.asks)
	return result
}

func (ob *OrderBook) GetSnapshot() *types.DepthUpdate {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bids := make([]types.Level, len(ob.bids))
	for i, l := range ob.bids {
		if l != nil {
			bids[i] = *l
		}
	}

	asks := make([]types.Level, len(ob.asks))
	for i, l := range ob.asks {
		if l != nil {
			asks[i] = *l
		}
	}

	return &types.DepthUpdate{
		Symbol:    ob.symbol,
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.Now().UnixMilli(),
	}
}

func (ob *OrderBook) Close() {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	ob.closed = true
	ob.bids = nil
	ob.asks = nil
	ob.orders = nil
}

var (
	ErrBookClosed    = &BookError{"order book is closed"}
	ErrOrderNotFound = &BookError{"order not found"}
)

type BookError struct {
	message string
}

func (e *BookError) Error() string {
	return e.message
}

func insertLevel(levels []*types.Level, level *types.Level, desc bool) []*types.Level {
	levels = append(levels, level)
	sort.Slice(levels, func(i, j int) bool {
		if desc {
			return levels[i].Price.GreaterThan(levels[j].Price)
		}
		return levels[i].Price.LessThan(levels[j].Price)
	})
	return levels
}

func removeLevel(levels []*types.Level, price decimal.Decimal) []*types.Level {
	for i, l := range levels {
		if l.Price.Equal(price) {
			return append(levels[:i], levels[i+1:]...)
		}
	}
	return levels
}

func copyLevelValues(levels []*types.Level) []types.Level {
	result := make([]types.Level, 0, len(levels))
	for _, level := range levels {
		if level != nil {
			result = append(result, *level)
		}
	}
	return result
}

func copyLevelPointers(levels []types.Level) []*types.Level {
	result := make([]*types.Level, 0, len(levels))
	for i := range levels {
		level := levels[i]
		result = append(result, &level)
	}
	return result
}

func copyOrderValues(orders map[string]*types.Order) []types.Order {
	ids := make([]string, 0, len(orders))
	for id := range orders {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	result := make([]types.Order, 0, len(ids))
	for _, id := range ids {
		if orders[id] != nil {
			result = append(result, *orders[id])
		}
	}
	return result
}
