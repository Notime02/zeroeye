package ws

import (
	"encoding/json"
	"testing"
)

func TestWebSocketOrderBookDeltaValidation(t *testing.T) {
	// Test malformed bid/ask price
	malformedPrice := []byte(`{"price": "abc"}`)
	if err := validateWebSocketOrderBookDelta(malformedPrice); err == nil {
		t.Errorf("expected error for malformed price, but got nil")
	}

	// Test malformed quantity
	malformedQuantity := []byte(`{"quantity": "abc"}`)
	if err := validateWebSocketOrderBookDelta(malformedQuantity); err == nil {
		t.Errorf("expected error for malformed quantity, but got nil")
	}

	// Test stale or out-of-order sequence updates
	staleSequence := []byte(`{"sequence": 1}`)
	if err := validateWebSocketOrderBookDelta(staleSequence); err == nil {
		t.Errorf("expected error for stale sequence, but got nil")
	}

	// Test valid snapshot followed by valid deltas
	validSnapshot := []byte(`{"snapshot": true}`)
	validDelta := []byte(`{"delta": true}`)
	if err := validateWebSocketOrderBookDelta(validSnapshot); err != nil {
		t.Errorf("expected no error for valid snapshot, but got %v", err)
	}
	if err := validateWebSocketOrderBookDelta(validDelta); err != nil {
		t.Errorf("expected no error for valid delta, but got %v", err)
	}
}

func validateWebSocketOrderBookDelta(delta []byte) error {
	var orderBookDelta struct {
		Price  string `json:"price"`
		Quantity string `json:"quantity"`
		Side    string `json:"side"`
		Symbol  string `json:"symbol"`
		Sequence int    `json:"sequence"`
	}
	if err := json.Unmarshal(delta, &orderBookDelta); err != nil {
		return err
	}
	if orderBookDelta.Price == "" || orderBookDelta.Quantity == "" || orderBookDelta.Side == "" || orderBookDelta.Symbol == "" {
		return errors.New("malformed order book delta")
	}
	if orderBookDelta.Sequence < 1 {
		return errors.New("stale or out-of-order sequence update")
	}
	return nil
}