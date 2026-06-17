package exchange

import (
	"context"
	"sort"
	"sync"

	"github.com/gorilla/websocket"
)

type LocalBook struct {
	mu   sync.RWMutex
	bids map[float64]float64
	asks map[float64]float64
}

func NewLocalBook() *LocalBook {
	return &LocalBook{
		bids: map[float64]float64{},
		asks: map[float64]float64{},
	}
}

func (b *LocalBook) ApplySnapshot(bids, asks [][]float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.bids = toMap(bids)
	b.asks = toMap(asks)
}

func (b *LocalBook) ApplyDelta(bids, asks [][]float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	applyDelta(b.bids, bids)
	applyDelta(b.asks, asks)
}

func (b *LocalBook) Lists() ([][]float64, [][]float64) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	bids := mapToLevels(b.bids, true)
	asks := mapToLevels(b.asks, false)
	return bids, asks
}

type Adapter interface {
	Run(ctx context.Context) error
}

type WebSocketAdapter struct {
	URL string
}

func (a WebSocketAdapter) Run(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, a.URL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	<-ctx.Done()
	return ctx.Err()
}

func toMap(levels [][]float64) map[float64]float64 {
	out := map[float64]float64{}
	for _, level := range levels {
		if len(level) != 2 {
			continue
		}
		out[level[0]] = level[1]
	}
	return out
}

func applyDelta(book map[float64]float64, levels [][]float64) {
	for _, level := range levels {
		if len(level) != 2 {
			continue
		}
		if level[1] == 0 {
			delete(book, level[0])
			continue
		}
		book[level[0]] = level[1]
	}
}

func mapToLevels(book map[float64]float64, descending bool) [][]float64 {
	out := make([][]float64, 0, len(book))
	for price, quantity := range book {
		out = append(out, []float64{price, quantity})
	}
	sort.Slice(out, func(i, j int) bool {
		if descending {
			return out[i][0] > out[j][0]
		}
		return out[i][0] < out[j][0]
	})
	return out
}
