package normalize

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
)

type Book struct {
	Symbol       string
	ExchangeTSMS contract.Millis
	Bids         [][]float64
	Asks         [][]float64
}

func LevelsJSON(levels [][]float64) (string, error) {
	payload, err := json.Marshal(levels)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func NormalizeLevels(value any, quantityIndex int) ([][]float64, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("levels must be array, got %T", value)
	}
	out := make([][]float64, 0, len(items))
	for i, item := range items {
		level, ok := item.([]any)
		if !ok {
			return nil, fmt.Errorf("level %d must be array, got %T", i, item)
		}
		if len(level) <= quantityIndex {
			return nil, fmt.Errorf("level %d has %d values, need index %d", i, len(level), quantityIndex)
		}
		price, err := number(level[0])
		if err != nil {
			return nil, fmt.Errorf("level %d price: %w", i, err)
		}
		quantity, err := number(level[quantityIndex])
		if err != nil {
			return nil, fmt.Errorf("level %d quantity: %w", i, err)
		}
		out = append(out, []float64{price, quantity})
	}
	return out, nil
}

func ExtractBook(row contract.RawPayload) (Book, bool) {
	bidsValue, asksValue, symbol, quantityIndex, ok := rawSides(row)
	if !ok {
		return Book{}, false
	}
	bids, err := NormalizeLevels(bidsValue, quantityIndex)
	if err != nil {
		return Book{}, false
	}
	asks, err := NormalizeLevels(asksValue, quantityIndex)
	if err != nil {
		return Book{}, false
	}
	if len(bids) == 0 || len(asks) == 0 {
		return Book{}, false
	}
	return Book{Symbol: symbol, Bids: bids, Asks: asks}, true
}

func ContainsOrderBookData(row contract.RawPayload) bool {
	bidsValue, asksValue, _, _, ok := rawSides(row)
	if !ok {
		return false
	}
	return levelCount(bidsValue) > 0 || levelCount(asksValue) > 0
}

func rawSides(row contract.RawPayload) (bids any, asks any, symbol string, quantityIndex int, ok bool) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
		return nil, nil, "", 1, false
	}

	switch row.Exchange {
	case "Binance":
		data, _ := payload["data"].(map[string]any)
		return data["b"], data["a"], payloadString(data, "s"), 1, true
	case "Bitget":
		data := firstObject(payload["data"])
		return data["bids"], data["asks"], payloadString(payloadMap(payload, "arg"), "instId"), 1, true
	case "Bybit":
		data := payloadMap(payload, "data")
		return data["b"], data["a"], payloadString(data, "s"), 1, true
	case "Gateio":
		result := payloadMap(payload, "result")
		return result["b"], result["a"], payloadString(result, "s"), 1, true
	case "Kucoin":
		data := payloadMap(payload, "data")
		return data["bids"], data["asks"], symbolAfterColon(payloadString(payload, "topic")), 1, true
	case "MEXC":
		data := payloadMap(payload, "data")
		quantityIndex := 1
		if row.InstrumentType == "future" || row.InstrumentType == "perp" {
			quantityIndex = 2
		}
		return data["bids"], data["asks"], payloadString(payload, "symbol"), quantityIndex, true
	case "OKX":
		data := firstObject(payload["data"])
		return data["bids"], data["asks"], payloadString(data, "instId"), 1, true
	default:
		return nil, nil, "", 1, false
	}
}

func firstObject(value any) map[string]any {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	out, _ := items[0].(map[string]any)
	return out
}

func payloadMap(value map[string]any, key string) map[string]any {
	out, _ := value[key].(map[string]any)
	return out
}

func payloadString(value map[string]any, key string) string {
	raw, _ := value[key].(string)
	return raw
}

func symbolAfterColon(topic string) string {
	for i := len(topic) - 1; i >= 0; i-- {
		if topic[i] == ':' {
			return topic[i+1:]
		}
	}
	return topic
}

func number(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case string:
		return strconv.ParseFloat(typed, 64)
	case json.Number:
		return typed.Float64()
	default:
		return 0, fmt.Errorf("unsupported number type %T", value)
	}
}

func levelCount(value any) int {
	items, ok := value.([]any)
	if !ok {
		return 0
	}
	return len(items)
}
