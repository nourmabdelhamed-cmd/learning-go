package samples

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestValidateSamplesReportsKnownFixtureShape(t *testing.T) {
	report, err := Validate(filepath.Join("..", "..", "samples"))
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if report.RawRows != 21 {
		t.Fatalf("raw rows = %d, want 21", report.RawRows)
	}
	if report.RawBookRows != 20 {
		t.Fatalf("raw book rows = %d, want 20", report.RawBookRows)
	}
	if report.NonBookRawRows != 1 {
		t.Fatalf("non-book raw rows = %d, want 1", report.NonBookRawRows)
	}
	if report.CEXNormalizedRows != 20 {
		t.Fatalf("cex normalized rows = %d, want 20", report.CEXNormalizedRows)
	}
	if report.SkippedDEXRows != 4 {
		t.Fatalf("skipped DEX rows = %d, want 4", report.SkippedDEXRows)
	}
	wantRawOnly := []string{"Kucoin|future|linear", "Kucoin|perp|linear"}
	if !reflect.DeepEqual(report.RawBookOnlyKeys, wantRawOnly) {
		t.Fatalf("raw-only keys = %#v, want %#v", report.RawBookOnlyKeys, wantRawOnly)
	}
	wantNormalizedOnly := []string{"Binance|spot|clob", "MEXC|spot|clob"}
	if !reflect.DeepEqual(report.NormalizedCEXOnlyKeys, wantNormalizedOnly) {
		t.Fatalf("normalized-only keys = %#v, want %#v", report.NormalizedCEXOnlyKeys, wantNormalizedOnly)
	}
}
