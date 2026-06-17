package samples

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/normalize"
)

const (
	RawSampleFile        = "BTC_raw_sample_all_exchanges_all_instruments.json"
	NormalizedSampleFile = "BTC_normalized_sample_all_exchanges_all_instruments.json"
)

type Report struct {
	RawRows               int      `json:"raw_rows"`
	RawBookRows           int      `json:"raw_book_rows"`
	NonBookRawRows        int      `json:"non_book_raw_rows"`
	NormalizedRows        int      `json:"normalized_rows"`
	CEXNormalizedRows     int      `json:"cex_normalized_rows"`
	SkippedDEXRows        int      `json:"skipped_dex_rows"`
	RawBookOnlyKeys       []string `json:"raw_book_only_keys"`
	NormalizedCEXOnlyKeys []string `json:"normalized_cex_only_keys"`
}

func LoadRaw(dir string) ([]contract.RawPayload, error) {
	var rows []contract.RawPayload
	if err := loadJSON(filepath.Join(dir, RawSampleFile), &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func LoadNormalized(dir string) ([]contract.NormalizedOrderBook, error) {
	var rows []contract.NormalizedOrderBook
	if err := loadJSON(filepath.Join(dir, NormalizedSampleFile), &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func CEXNormalized(rows []contract.NormalizedOrderBook) (kept []contract.NormalizedOrderBook, skippedDEX int) {
	for _, row := range rows {
		if row.IsDEX() {
			skippedDEX++
			continue
		}
		kept = append(kept, row)
	}
	return kept, skippedDEX
}

func Validate(dir string) (Report, error) {
	rawRows, err := LoadRaw(dir)
	if err != nil {
		return Report{}, err
	}
	normalizedRows, err := LoadNormalized(dir)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		RawRows:        len(rawRows),
		NormalizedRows: len(normalizedRows),
	}

	rawBookKeys := map[string]struct{}{}
	for _, row := range rawRows {
		if normalize.ContainsOrderBookData(row) {
			report.RawBookRows++
			rawBookKeys[key(row.Exchange, row.InstrumentType, row.ProductType)] = struct{}{}
		} else {
			report.NonBookRawRows++
		}
	}

	normalizedCEX, skippedDEX := CEXNormalized(normalizedRows)
	report.CEXNormalizedRows = len(normalizedCEX)
	report.SkippedDEXRows = skippedDEX
	normalizedKeys := map[string]struct{}{}
	for _, row := range normalizedCEX {
		if err := row.ValidateBookJSON(); err != nil {
			return Report{}, fmt.Errorf("%s %s %s: %w", row.Exchange, row.InstrumentType, row.InstrumentID, err)
		}
		normalizedKeys[key(row.Exchange, row.InstrumentType, row.ProductType)] = struct{}{}
	}

	report.RawBookOnlyKeys = missingFrom(rawBookKeys, normalizedKeys)
	report.NormalizedCEXOnlyKeys = missingFrom(normalizedKeys, rawBookKeys)
	return report, nil
}

func loadJSON(path string, out any) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	payload = bytes.TrimPrefix(payload, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func key(exchange, instrumentType, productType string) string {
	return exchange + "|" + instrumentType + "|" + productType
}

func missingFrom(left, right map[string]struct{}) []string {
	var out []string
	for value := range left {
		if _, ok := right[value]; !ok {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}
