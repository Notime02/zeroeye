package ws

import (
	"encoding/json"
	"errors"
)

func validateOrderBookDelta(deltaJSON []byte) (map[string]interface{}, error) {
	var delta map[string]interface{}
	if err := json.Unmarshal(deltaJSON, &delta); err != nil {
		return nil, err
	}

	price, ok := delta["price"]
	if !ok || price == nil {
		return nil, errors.New("missing price")
	}
	priceFloat, ok := price.(float64)
	if !ok {
		return nil, errors.New("invalid price")
	}
	if priceFloat <= 0 {
		return nil, errors.New("invalid price")
	}

	quantity, ok := delta["quantity"]
	if !ok || quantity == nil {
		return nil, errors.New("missing quantity")
	}
	quantityFloat, ok := quantity.(float64)
	if !ok {
		return nil, errors.New("invalid quantity")
	}
	if quantityFloat <= 0 {
		return nil, errors.New("invalid quantity")
	}

	side, ok := delta["side"]
	if !ok || side == nil {
		return nil, errors.New("missing side")
	}
	sideStr, ok := side.(string)
	if !ok {
		return nil, errors.New("invalid side")
	}
	if sideStr != "buy" && sideStr != "sell" {
		return nil, errors.New("invalid side")
	}

	symbol, ok := delta["symbol"]
	if !ok || symbol == nil {
		return nil, errors.New("missing symbol")
	}
	_, ok = symbol.(string)
	if !ok {
		return nil, errors.New("invalid symbol")
	}

	sequence, ok := delta["sequence"]
	if ok && sequence != nil {
		sequenceInt, ok := sequence.(float64)
		if !ok {
			return nil, errors.New("invalid sequence")
		}
		if sequenceInt < 0 {
			return nil, errors.New("stale delta")
		}
	}

	return delta, nil
}
