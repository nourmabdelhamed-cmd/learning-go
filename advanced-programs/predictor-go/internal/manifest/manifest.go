package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
)

type Manifest struct {
	Version     string                    `json:"version"`
	Source      string                    `json:"source"`
	Instruments []contract.InstrumentSpec `json:"instruments"`
}

var SupportedCEX = map[string]map[string]bool{
	"Binance": {"spot": true, "perp": true, "future": true, "option": true},
	"Bybit":   {"spot": true, "perp": true, "future": true, "option": true},
	"OKX":     {"spot": true, "perp": true, "future": true, "option": true},
	"Bitget":  {"spot": true, "perp": true, "future": true},
	"MEXC":    {"spot": true, "perp": true, "future": true},
	"Kucoin":  {"spot": true, "perp": true, "future": true},
	"Gateio":  {"spot": true},
}

func Load(path string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()

	var manifest Manifest
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if err := Validate(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func Validate(manifest Manifest) error {
	if strings.TrimSpace(manifest.Version) == "" {
		return fmt.Errorf("manifest version must not be empty")
	}
	if len(manifest.Instruments) == 0 {
		return fmt.Errorf("manifest instruments must not be empty")
	}

	seen := map[string]struct{}{}
	for i, spec := range manifest.Instruments {
		if err := ValidateSpec(spec); err != nil {
			return fmt.Errorf("instrument %d: %w", i, err)
		}
		key := contract.ManifestKey(spec)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate instrument %q", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func ValidateSpec(spec contract.InstrumentSpec) error {
	required := map[string]string{
		"exchange":        spec.Exchange,
		"instrument_type": spec.InstrumentType,
		"product_type":    spec.ProductType,
		"symbol":          spec.Symbol,
		"base":            spec.Base,
		"quote":           spec.Quote,
		"instrument_id":   spec.InstrumentID,
		"venue_symbol":    spec.VenueSymbol,
	}
	for field, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not be empty", field)
		}
	}
	if spec.Enabled == false {
		return fmt.Errorf("disabled instruments should be omitted from the learning manifest")
	}
	supportedTypes, ok := SupportedCEX[spec.Exchange]
	if !ok {
		return fmt.Errorf("unsupported exchange %q", spec.Exchange)
	}
	if !supportedTypes[spec.InstrumentType] {
		return fmt.Errorf("unsupported instrument type %q for %s", spec.InstrumentType, spec.Exchange)
	}
	return nil
}

func (m Manifest) ByExchange() map[string][]contract.InstrumentSpec {
	out := map[string][]contract.InstrumentSpec{}
	for _, spec := range m.Instruments {
		out[spec.Exchange] = append(out[spec.Exchange], spec)
	}
	for exchange := range out {
		sort.Slice(out[exchange], func(i, j int) bool {
			return contract.ManifestKey(out[exchange][i]) < contract.ManifestKey(out[exchange][j])
		})
	}
	return out
}
