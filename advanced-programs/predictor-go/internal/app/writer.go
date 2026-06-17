package app

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/broker"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/contract"
	"github.com/tannergabriel/learning-go/advanced-programs/predictor-go/internal/storage"
)

type WriterService struct {
	Consumer    broker.Consumer
	Writer      *storage.BufferedWriter
	ReadCount   int64
	Block       time.Duration
	ChannelSize int
}

func (s WriterService) Run(ctx context.Context) error {
	if s.ReadCount <= 0 {
		s.ReadCount = 100
	}
	if s.Block <= 0 {
		s.Block = time.Second
	}
	if s.ChannelSize <= 0 {
		s.ChannelSize = 256
	}

	messages := make(chan broker.StreamMessage, s.ChannelSize)
	errs := make(chan error, 1)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(messages)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			batch, err := s.Consumer.Read(ctx, s.ReadCount, s.Block)
			if err != nil {
				select {
				case errs <- err:
				default:
				}
				return
			}
			for _, message := range batch {
				select {
				case messages <- message:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for message := range messages {
			if err := s.writeMessage(ctx, message); err != nil {
				select {
				case errs <- err:
				default:
				}
				return
			}
			if err := s.Consumer.Ack(ctx, []broker.StreamMessage{message}); err != nil {
				select {
				case errs <- err:
				default:
				}
				return
			}
		}
		if err := s.Writer.Flush(ctx); err != nil {
			select {
			case errs <- err:
			default:
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		<-done
		return ctx.Err()
	case err := <-errs:
		return err
	case <-done:
		return nil
	}
}

func (s WriterService) writeMessage(ctx context.Context, message broker.StreamMessage) error {
	switch message.Kind {
	case broker.RawMessage:
		row, err := contract.RawPayloadFromFields(message.Fields)
		if err != nil {
			return err
		}
		row.StreamID = message.ID
		return s.Writer.AddRaw(ctx, row)
	case broker.NormalizedMessage:
		row, err := contract.NormalizedOrderBookFromFields(message.Fields)
		if err != nil {
			return err
		}
		row.StreamID = message.ID
		return s.Writer.AddNormalized(ctx, row)
	default:
		log.Printf("skipping unknown message kind %q", message.Kind)
		return nil
	}
}
