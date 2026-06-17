package contract

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	SchemaVersion             = "2"
	RawRedisStream            = "orderbook.raw.v1"
	NormalizedRedisStream     = "orderbook.normalized.v1"
	InstrumentManifestKey     = "orderbook.instrument_manifest.v1"
	NormalizedClickHouseTable = "normalized_orderbook_snapshots"
	RawClickHouseTable        = "raw_orderbook_payloads"
)

var DEXExchanges = map[string]struct{}{
	"dydx":        {},
	"pancakeswap": {},
	"sushiswap":   {},
	"uniswap":     {},
}

type Millis int64

func NowMillis() Millis {
	return Millis(time.Now().UnixMilli())
}

func (m Millis) Int64() int64 {
	return int64(m)
}

func (m Millis) String() string {
	return strconv.FormatInt(int64(m), 10)
}

func (m Millis) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(m), 10)), nil
}

func (m *Millis) UnmarshalJSON(payload []byte) error {
	value := strings.TrimSpace(string(payload))
	value = strings.Trim(value, `"`)
	if value == "" || value == "null" {
		*m = 0
		return nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("parse millis %q: %w", value, err)
	}
	*m = Millis(parsed)
	return nil
}

type RawPayload struct {
	SchemaVersion    string `json:"schema_version,omitempty"`
	Exchange         string `json:"exchange"`
	InstrumentType   string `json:"instrument_type"`
	ProductType      string `json:"product_type"`
	ConnectionID     string `json:"connection_id"`
	ReceivedTSMS     Millis `json:"received_ts_ms"`
	PayloadFormat    string `json:"payload_format"`
	PayloadTruncated string `json:"payload_truncated"`
	StreamID         string `json:"stream_id"`
	Payload          string `json:"payload"`
}

type NormalizedOrderBook struct {
	SchemaVersion  string `json:"schema_version,omitempty"`
	ProductType    string `json:"product_type"`
	InstrumentType string `json:"instrument_type"`
	Exchange       string `json:"exchange"`
	Symbol         string `json:"symbol"`
	Base           string `json:"base"`
	Quote          string `json:"quote"`
	Underlying     string `json:"underlying"`
	InstrumentID   string `json:"instrument_id"`
	ReceivedTSMS   Millis `json:"received_ts_ms"`
	StreamID       string `json:"stream_id"`
	BidsJSON       string `json:"bids_json"`
	AsksJSON       string `json:"asks_json"`
}

type InstrumentSpec struct {
	Exchange       string `json:"exchange"`
	InstrumentType string `json:"instrument_type"`
	ProductType    string `json:"product_type"`
	Symbol         string `json:"symbol"`
	Base           string `json:"base"`
	Quote          string `json:"quote"`
	Underlying     string `json:"underlying"`
	InstrumentID   string `json:"instrument_id"`
	VenueSymbol    string `json:"venue_symbol"`
	Endpoint       string `json:"endpoint,omitempty"`
	Depth          int    `json:"depth,omitempty"`
	Enabled        bool   `json:"enabled"`
}

func (r RawPayload) RedisFields() map[string]any {
	schemaVersion := r.SchemaVersion
	if schemaVersion == "" {
		schemaVersion = SchemaVersion
	}
	return map[string]any{
		"schema_version":    schemaVersion,
		"exchange":          r.Exchange,
		"instrument_type":   r.InstrumentType,
		"product_type":      r.ProductType,
		"connection_id":     r.ConnectionID,
		"received_ts_ms":    r.ReceivedTSMS.String(),
		"payload_format":    r.PayloadFormat,
		"payload_truncated": r.PayloadTruncated,
		"stream_id":         r.StreamID,
		"payload":           r.Payload,
	}
}

func RawPayloadFromFields(fields map[string]string) (RawPayload, error) {
	received, err := parseMillisField(fields["received_ts_ms"])
	if err != nil {
		return RawPayload{}, err
	}
	return RawPayload{
		SchemaVersion:    defaultString(fields["schema_version"], SchemaVersion),
		Exchange:         fields["exchange"],
		InstrumentType:   fields["instrument_type"],
		ProductType:      fields["product_type"],
		ConnectionID:     fields["connection_id"],
		ReceivedTSMS:     received,
		PayloadFormat:    fields["payload_format"],
		PayloadTruncated: fields["payload_truncated"],
		StreamID:         fields["stream_id"],
		Payload:          fields["payload"],
	}, nil
}

func (n NormalizedOrderBook) RedisFields() map[string]any {
	schemaVersion := n.SchemaVersion
	if schemaVersion == "" {
		schemaVersion = SchemaVersion
	}
	return map[string]any{
		"schema_version":  schemaVersion,
		"product_type":    n.ProductType,
		"instrument_type": n.InstrumentType,
		"exchange":        n.Exchange,
		"symbol":          n.Symbol,
		"base":            n.Base,
		"quote":           n.Quote,
		"underlying":      n.Underlying,
		"instrument_id":   n.InstrumentID,
		"received_ts_ms":  n.ReceivedTSMS.String(),
		"stream_id":       n.StreamID,
		"bids_json":       n.BidsJSON,
		"asks_json":       n.AsksJSON,
	}
}

func NormalizedOrderBookFromFields(fields map[string]string) (NormalizedOrderBook, error) {
	received, err := parseMillisField(fields["received_ts_ms"])
	if err != nil {
		return NormalizedOrderBook{}, err
	}
	return NormalizedOrderBook{
		SchemaVersion:  defaultString(fields["schema_version"], SchemaVersion),
		ProductType:    fields["product_type"],
		InstrumentType: fields["instrument_type"],
		Exchange:       fields["exchange"],
		Symbol:         fields["symbol"],
		Base:           fields["base"],
		Quote:          fields["quote"],
		Underlying:     fields["underlying"],
		InstrumentID:   fields["instrument_id"],
		ReceivedTSMS:   received,
		StreamID:       fields["stream_id"],
		BidsJSON:       fields["bids_json"],
		AsksJSON:       fields["asks_json"],
	}, nil
}

func (n NormalizedOrderBook) IsDEX() bool {
	if strings.EqualFold(n.InstrumentType, "dex") || strings.EqualFold(n.ProductType, "amm") {
		return true
	}
	_, ok := DEXExchanges[strings.ToLower(n.Exchange)]
	return ok
}

func (n NormalizedOrderBook) ValidateBookJSON() error {
	if err := validateLevelsJSON(n.BidsJSON); err != nil {
		return fmt.Errorf("bids_json: %w", err)
	}
	if err := validateLevelsJSON(n.AsksJSON); err != nil {
		return fmt.Errorf("asks_json: %w", err)
	}
	return nil
}

func ManifestKey(spec InstrumentSpec) string {
	return strings.Join([]string{
		spec.Exchange,
		spec.InstrumentType,
		spec.ProductType,
		spec.InstrumentID,
	}, "|")
}

func parseMillisField(value string) (Millis, error) {
	var out Millis
	if err := out.UnmarshalJSON([]byte(strconv.Quote(value))); err != nil {
		return 0, err
	}
	return out, nil
}

func validateLevelsJSON(value string) error {
	var levels [][]float64
	if err := json.Unmarshal([]byte(value), &levels); err != nil {
		return err
	}
	for i, level := range levels {
		if len(level) != 2 {
			return fmt.Errorf("level %d has %d values, want 2", i, len(level))
		}
	}
	return nil
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
