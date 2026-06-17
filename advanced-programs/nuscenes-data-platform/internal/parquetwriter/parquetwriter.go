package parquetwriter

import (
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress/snappy"
)

func Write[T any](path string, rows []T, options ...parquet.WriterOption) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	writerOptions := []parquet.WriterOption{parquet.Compression(&snappy.Codec{})}
	writerOptions = append(writerOptions, options...)
	return parquet.WriteFile(path, rows, writerOptions...)
}

func Read[T any](path string) ([]T, error) {
	return parquet.ReadFile[T](path)
}

type Stream[T any] struct {
	file   *os.File
	writer *parquet.GenericWriter[T]
}

func NewStream[T any](path string, options ...parquet.WriterOption) (*Stream[T], error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	writerOptions := []parquet.WriterOption{parquet.Compression(&snappy.Codec{})}
	writerOptions = append(writerOptions, options...)
	return &Stream[T]{
		file:   file,
		writer: parquet.NewGenericWriter[T](file, writerOptions...),
	}, nil
}

func (s *Stream[T]) Write(rows []T) error {
	if len(rows) == 0 {
		return nil
	}
	if _, err := s.writer.Write(rows); err != nil {
		return err
	}
	return s.writer.Flush()
}

func (s *Stream[T]) Close() error {
	writerErr := s.writer.Close()
	fileErr := s.file.Close()
	if writerErr != nil {
		return writerErr
	}
	return fileErr
}
