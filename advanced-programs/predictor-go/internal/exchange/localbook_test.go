package exchange

import "testing"

func TestLocalBookAppliesSnapshotAndDelta(t *testing.T) {
	book := NewLocalBook()
	book.ApplySnapshot([][]float64{{1, 2}}, [][]float64{{1.1, 3}})
	book.ApplyDelta([][]float64{{0.9, 4}, {1, 0}}, [][]float64{{1.2, 5}})
	bids, asks := book.Lists()
	if len(bids) != 1 || bids[0][0] != 0.9 {
		t.Fatalf("bids = %#v", bids)
	}
	if len(asks) != 2 || asks[0][0] != 1.1 || asks[1][0] != 1.2 {
		t.Fatalf("asks = %#v", asks)
	}
}
