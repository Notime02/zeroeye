package orderbook

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/tent-of-trials/market/types"
)

func TestSnapshotRecoverRoundTripIsDeterministic(t *testing.T) {
	book := NewOrderBook(types.Symbol("BTC-USD"), Config{MaxDepth: 10, PriceDecimals: 8, VolumeDecimals: 8})
	orders := []*types.Order{
		{
			ID:           "order-b",
			Symbol:       types.Symbol("BTC-USD"),
			Side:         types.Sell,
			Type:         types.Limit,
			Price:        decimal.RequireFromString("102.25"),
			Quantity:     decimal.RequireFromString("1.5"),
			RemainingQty: decimal.RequireFromString("1.5"),
		},
		{
			ID:           "order-a",
			Symbol:       types.Symbol("BTC-USD"),
			Side:         types.Buy,
			Type:         types.Limit,
			Price:        decimal.RequireFromString("101.10"),
			Quantity:     decimal.RequireFromString("2.0"),
			RemainingQty: decimal.RequireFromString("2.0"),
		},
	}

	for _, order := range orders {
		if _, err := book.AddOrder(order); err != nil {
			t.Fatalf("add order: %v", err)
		}
	}

	first, err := book.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	second, err := book.Snapshot()
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("snapshot is not deterministic:\n%s\n%s", first, second)
	}

	recovered := NewOrderBook(types.Symbol("BTC-USD"), Config{MaxDepth: 1})
	if err := recovered.Recover(first); err != nil {
		t.Fatalf("recover: %v", err)
	}
	roundTrip, err := recovered.Snapshot()
	if err != nil {
		t.Fatalf("round trip snapshot: %v", err)
	}
	if string(first) != string(roundTrip) {
		t.Fatalf("round trip snapshot mismatch:\n%s\n%s", first, roundTrip)
	}
}

func TestRecoverRejectsDuplicateOrderIDs(t *testing.T) {
	book := NewOrderBook(types.Symbol("BTC-USD"), Config{MaxDepth: 10})
	err := book.Recover([]byte(`{
		"version":1,
		"symbol":"BTC-USD",
		"config":{"MaxDepth":10,"PriceDecimals":0,"VolumeDecimals":0},
		"sequence":1,
		"updated_at":"0001-01-01T00:00:00Z",
		"bids":[],
		"asks":[],
		"orders":[
			{"id":"same","symbol":"BTC-USD"},
			{"id":"same","symbol":"BTC-USD"}
		]
	}`))
	if err == nil {
		t.Fatal("expected duplicate order id error")
	}
}
