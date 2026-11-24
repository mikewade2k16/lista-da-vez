package panics

import (
	"sync"
	"testing"
)

func TestRecordConcurrent(t *testing.T) {
	before := Total()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Record("test")
		}()
	}
	wg.Wait()
	if got := Total() - before; got != 50 {
		t.Fatalf("got %d, want 50", got)
	}
}
