package normalize

import (
	"encoding/json"
	"testing"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
)

func TestNormalizeLevelsUsesQuantityIndex(t *testing.T) {
	var levels any
	if err := json.Unmarshal([]byte(`[[76575.1,6585,2]]`), &levels); err != nil {
		t.Fatal(err)
	}
	got, err := NormalizeLevels(levels, 2)
	if err != nil {
		t.Fatalf("NormalizeLevels returned error: %v", err)
	}
	if got[0][0] != 76575.1 || got[0][1] != 2 {
		t.Fatalf("levels = %#v", got)
	}
}

func TestExtractBookDetectsBookAndAckPayloads(t *testing.T) {
	bookRow := contract.RawPayload{
		Exchange: "Binance",
		Payload:  `{"stream":"btcusdt@depth20@100ms","data":{"s":"BTCUSDT","b":[["1","2"]],"a":[["1.1","3"]]}}`,
	}
	if book, ok := ExtractBook(bookRow); !ok || book.Symbol != "BTCUSDT" || len(book.Bids) != 1 {
		t.Fatalf("book = %#v ok=%v", book, ok)
	}

	ackRow := contract.RawPayload{
		Exchange: "MEXC",
		Payload:  `{"id":0,"code":0,"msg":"subscribed"}`,
	}
	if _, ok := ExtractBook(ackRow); ok {
		t.Fatal("subscription ack should not be treated as an order book")
	}
}

func TestContainsOrderBookDataAllowsOneSidedDeltas(t *testing.T) {
	row := contract.RawPayload{
		Exchange: "Bybit",
		Payload:  `{"topic":"orderbook.50.BTCUSDT","data":{"s":"BTCUSDT","b":[["1","2"]],"a":[]}}`,
	}
	if !ContainsOrderBookData(row) {
		t.Fatal("expected one-sided delta to count as raw order-book data")
	}
	if _, ok := ExtractBook(row); ok {
		t.Fatal("one-sided delta should not extract as complete normalized book")
	}
}
