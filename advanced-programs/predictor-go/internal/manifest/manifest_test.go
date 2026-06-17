package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSampleManifest(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "btc-instruments.sample.json")
	manifest, err := Load(path)
	if err != nil {
		t.Fatalf("Load sample manifest: %v", err)
	}
	if len(manifest.Instruments) != 20 {
		t.Fatalf("instruments = %d, want 20", len(manifest.Instruments))
	}
	if len(manifest.ByExchange()["OKX"]) != 4 {
		t.Fatalf("OKX instruments = %d, want 4", len(manifest.ByExchange()["OKX"]))
	}
}

func TestLoadRejectsUnknownFieldsAndUnsupportedExchange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	payload := `{
  "version": "bad",
  "source": "test",
  "instruments": [{
    "exchange": "uniswap",
    "instrument_type": "dex",
    "product_type": "amm",
    "symbol": "BTCUSDT",
    "base": "BTC",
    "quote": "USDT",
    "underlying": "BTC",
    "instrument_id": "dex:ethereum:uniswap:BTC-USDT",
    "venue_symbol": "BTC-USDT",
    "enabled": true
  }]
}`
	if err := os.WriteFile(path, []byte(payload), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected unsupported exchange error")
	}
	if !strings.Contains(err.Error(), "unsupported exchange") {
		t.Fatalf("error = %q", err)
	}
}
