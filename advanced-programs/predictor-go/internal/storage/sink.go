package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/parquet-go/parquet-go"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
)

type Sink interface {
	WriteRaw(ctx context.Context, rows []contract.RawPayload) error
	WriteNormalized(ctx context.Context, rows []contract.NormalizedOrderBook) error
	Close() error
}

type MemorySink struct {
	Raw        []contract.RawPayload
	Normalized []contract.NormalizedOrderBook
}

func (s *MemorySink) WriteRaw(_ context.Context, rows []contract.RawPayload) error {
	s.Raw = append(s.Raw, rows...)
	return nil
}

func (s *MemorySink) WriteNormalized(_ context.Context, rows []contract.NormalizedOrderBook) error {
	s.Normalized = append(s.Normalized, rows...)
	return nil
}

func (s *MemorySink) Close() error {
	return nil
}

type ParquetSink struct {
	outputDir string
}

func NewParquetSink(outputDir string) *ParquetSink {
	return &ParquetSink{outputDir: outputDir}
}

func (s *ParquetSink) WriteRaw(_ context.Context, rows []contract.RawPayload) error {
	if len(rows) == 0 {
		return nil
	}
	path := partitionPath(s.outputDir, "raw", rows[0].Exchange)
	return parquet.WriteFile(path, rows)
}

func (s *ParquetSink) WriteNormalized(_ context.Context, rows []contract.NormalizedOrderBook) error {
	if len(rows) == 0 {
		return nil
	}
	path := partitionPath(s.outputDir, "normalized", rows[0].Exchange)
	return parquet.WriteFile(path, rows)
}

func (s *ParquetSink) Close() error {
	return nil
}

type ClickHouseSink struct {
	conn      clickhouse.Conn
	database  string
	rawTable  string
	normTable string
}

type ClickHouseConfig struct {
	Addr      []string
	Database  string
	Username  string
	Password  string
	RawTable  string
	NormTable string
}

func OpenClickHouse(ctx context.Context, cfg ClickHouseConfig) (*ClickHouseSink, error) {
	if len(cfg.Addr) == 0 {
		cfg.Addr = []string{"localhost:9000"}
	}
	if cfg.Database == "" {
		cfg.Database = "orderbooks"
	}
	if cfg.Username == "" {
		cfg.Username = "default"
	}
	if cfg.RawTable == "" {
		cfg.RawTable = contract.RawClickHouseTable
	}
	if cfg.NormTable == "" {
		cfg.NormTable = contract.NormalizedClickHouseTable
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: cfg.Addr,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
	})
	if err != nil {
		return nil, err
	}
	sink := &ClickHouseSink{
		conn:      conn,
		database:  cfg.Database,
		rawTable:  cfg.RawTable,
		normTable: cfg.NormTable,
	}
	if err := sink.EnsureSchema(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return sink, nil
}

func (s *ClickHouseSink) EnsureSchema(ctx context.Context) error {
	if err := s.conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", s.database)); err != nil {
		return err
	}
	if err := s.conn.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s
(
    schema_version LowCardinality(String),
    exchange LowCardinality(String),
    instrument_type LowCardinality(String),
    product_type LowCardinality(String),
    connection_id String,
    received_ts_ms Int64,
    payload_format LowCardinality(String),
    payload_truncated LowCardinality(String),
    stream_id String,
    payload String,
    consumed_ts_ms Int64,
    written_batch_ts_ms Int64,
    received_at DateTime64(3, 'UTC') MATERIALIZED fromUnixTimestamp64Milli(received_ts_ms, 'UTC'),
    written_at DateTime64(3, 'UTC') MATERIALIZED fromUnixTimestamp64Milli(written_batch_ts_ms, 'UTC')
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(received_at)
ORDER BY (exchange, instrument_type, product_type, received_at, stream_id)
`, s.qualified(s.rawTable))); err != nil {
		return err
	}
	if err := s.conn.Exec(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s
(
    schema_version LowCardinality(String),
    product_type LowCardinality(String),
    instrument_type LowCardinality(String),
    exchange LowCardinality(String),
    symbol LowCardinality(String),
    base LowCardinality(String),
    quote LowCardinality(String),
    underlying String,
    instrument_id String,
    received_ts_ms Int64,
    stream_id String,
    bids_json String,
    asks_json String,
    consumed_ts_ms Int64,
    written_batch_ts_ms Int64,
    received_at DateTime64(3, 'UTC') MATERIALIZED fromUnixTimestamp64Milli(received_ts_ms, 'UTC'),
    written_at DateTime64(3, 'UTC') MATERIALIZED fromUnixTimestamp64Milli(written_batch_ts_ms, 'UTC')
)
ENGINE = MergeTree
PARTITION BY toYYYYMMDD(received_at)
ORDER BY (exchange, instrument_type, product_type, instrument_id, received_at, stream_id)
`, s.qualified(s.normTable))); err != nil {
		return err
	}
	for _, column := range normalizedAlterColumns {
		if err := s.conn.Exec(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s", s.qualified(s.normTable), column)); err != nil {
			return err
		}
	}
	return nil
}

func (s *ClickHouseSink) WriteRaw(ctx context.Context, rows []contract.RawPayload) error {
	if len(rows) == 0 {
		return nil
	}
	now := contract.NowMillis().Int64()
	batch, err := s.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s (%s)", s.qualified(s.rawTable), rawInsertColumns))
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.Append(
			defaultString(row.SchemaVersion, contract.SchemaVersion),
			row.Exchange,
			row.InstrumentType,
			row.ProductType,
			row.ConnectionID,
			row.ReceivedTSMS.Int64(),
			row.PayloadFormat,
			row.PayloadTruncated,
			row.StreamID,
			row.Payload,
			now,
			now,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *ClickHouseSink) WriteNormalized(ctx context.Context, rows []contract.NormalizedOrderBook) error {
	if len(rows) == 0 {
		return nil
	}
	now := contract.NowMillis().Int64()
	batch, err := s.conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO %s (%s)", s.qualified(s.normTable), normalizedInsertColumns))
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.Append(
			defaultString(row.SchemaVersion, contract.SchemaVersion),
			row.ProductType,
			row.InstrumentType,
			row.Exchange,
			row.Symbol,
			row.Base,
			row.Quote,
			row.Underlying,
			row.InstrumentID,
			row.ReceivedTSMS.Int64(),
			row.StreamID,
			row.BidsJSON,
			row.AsksJSON,
			now,
			now,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *ClickHouseSink) Close() error {
	return s.conn.Close()
}

func (s *ClickHouseSink) qualified(table string) string {
	return fmt.Sprintf("`%s`.`%s`", s.database, table)
}

var normalizedAlterColumns = []string{
	"stream_id String",
	"product_type LowCardinality(String)",
	"instrument_type LowCardinality(String)",
	"base LowCardinality(String)",
	"quote LowCardinality(String)",
	"underlying String",
	"instrument_id String",
}

const rawInsertColumns = `
    schema_version,
    exchange,
    instrument_type,
    product_type,
    connection_id,
    received_ts_ms,
    payload_format,
    payload_truncated,
    stream_id,
    payload,
    consumed_ts_ms,
    written_batch_ts_ms
`

const normalizedInsertColumns = `
    schema_version,
    product_type,
    instrument_type,
    exchange,
    symbol,
    base,
    quote,
    underlying,
    instrument_id,
    received_ts_ms,
    stream_id,
    bids_json,
    asks_json,
    consumed_ts_ms,
    written_batch_ts_ms
`

func partitionPath(root, kind, exchange string) string {
	now := time.Now().UTC()
	dir := filepath.Join(root, kind, "date="+now.Format("2006-01-02"), "hour="+now.Format("15"), "exchange="+exchange)
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, fmt.Sprintf("%d.parquet", now.UnixNano()))
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
