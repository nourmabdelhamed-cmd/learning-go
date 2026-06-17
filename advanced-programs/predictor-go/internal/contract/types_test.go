package contract

import (
	"encoding/json"
	"testing"
)

func TestMillisUnmarshalAcceptsStringAndNumber(t *testing.T) {
	var fromString struct {
		T Millis `json:"t"`
	}
	if err := json.Unmarshal([]byte(`{"t":"1779126020112"}`), &fromString); err != nil {
		t.Fatal(err)
	}
	var fromNumber struct {
		T Millis `json:"t"`
	}
	if err := json.Unmarshal([]byte(`{"t":1779126020112}`), &fromNumber); err != nil {
		t.Fatal(err)
	}
	if fromString.T != fromNumber.T {
		t.Fatalf("millis mismatch: %d != %d", fromString.T, fromNumber.T)
	}
}

func TestNormalizedRoundTripAndValidation(t *testing.T) {
	event := NormalizedOrderBook{
		ProductType:    "linear",
		InstrumentType: "perp",
		Exchange:       "Binance",
		Symbol:         "BTCUSDT",
		Base:           "BTC",
		Quote:          "USDT",
		InstrumentID:   "BTCUSDT",
		ReceivedTSMS:   100,
		StreamID:       "100-0",
		BidsJSON:       `[[1,2]]`,
		AsksJSON:       `[[1.1,3]]`,
	}
	restored, err := NormalizedOrderBookFromFields(stringFields(event.RedisFields()))
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if restored.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %q, want %q", restored.SchemaVersion, SchemaVersion)
	}
	if err := restored.ValidateBookJSON(); err != nil {
		t.Fatalf("ValidateBookJSON returned error: %v", err)
	}
}

func TestDetectsDEXRows(t *testing.T) {
	row := NormalizedOrderBook{Exchange: "uniswap", InstrumentType: "dex", ProductType: "amm"}
	if !row.IsDEX() {
		t.Fatal("expected DEX row")
	}
}

func stringFields(fields map[string]any) map[string]string {
	out := make(map[string]string, len(fields))
	for key, value := range fields {
		out[key] = value.(string)
	}
	return out
}
