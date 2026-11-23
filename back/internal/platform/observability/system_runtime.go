package observability

import (
	"runtime"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type goMetrics struct {
	Goroutines     int     `json:"goroutines"`
	HeapAllocBytes uint64  `json:"heapAllocBytes"`
	HeapSysBytes   uint64  `json:"heapSysBytes"`
	NumGC          uint32  `json:"numGC"`
	GCPauseP99Ms   float64 `json:"gcPauseP99Ms"`
	GOMAXPROCS     int     `json:"gomaxprocs"`
}

func readGoMetrics() goMetrics {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return goMetrics{
		Goroutines:     runtime.NumGoroutine(),
		HeapAllocBytes: mem.HeapAlloc,
		HeapSysBytes:   mem.HeapSys,
		NumGC:          mem.NumGC,
		GCPauseP99Ms:   gcPauseP99(mem.PauseNs[:], mem.NumGC),
		GOMAXPROCS:     runtime.GOMAXPROCS(0),
	}
}

// gcPauseP99 usa o buffer circular de pausas (256 últimas) e devolve ms.
func gcPauseP99(pauses []uint64, numGC uint32) float64 {
	count := min(int(numGC), len(pauses))
	if count == 0 {
		return 0
	}
	values := make([]uint64, 0, count)
	for i := range count {
		values = append(values, pauses[i])
	}
	sort.Slice(values, func(a, b int) bool { return values[a] < values[b] })
	index := min(count-1, int(float64(count)*0.99))
	return float64(time.Duration(values[index])) / float64(time.Millisecond)
}

type poolMetrics struct {
	Max                  int32   `json:"max"`
	Total                int32   `json:"total"`
	Acquired             int32   `json:"acquired"`
	Idle                 int32   `json:"idle"`
	AcquireCountDelta    int64   `json:"acquireCountDelta"`
	EmptyAcquireDelta    int64   `json:"emptyAcquireDelta"`
	CanceledAcquireDelta int64   `json:"canceledAcquireDelta"`
	AcquireWaitMsDelta   float64 `json:"acquireWaitMsDelta"`
	Utilization          float64 `json:"utilization"`
}

type poolCounters struct {
	max, total, acquired, idle int32
	acquireCount               int64
	emptyAcquire               int64
	canceledAcquire            int64
	acquireDuration            time.Duration
}

func readPoolCounters(pool *pgxpool.Pool) poolCounters {
	stat := pool.Stat()
	return poolCounters{
		max:             stat.MaxConns(),
		total:           stat.TotalConns(),
		acquired:        stat.AcquiredConns(),
		idle:            stat.IdleConns(),
		acquireCount:    stat.AcquireCount(),
		emptyAcquire:    stat.EmptyAcquireCount(),
		canceledAcquire: stat.CanceledAcquireCount(),
		acquireDuration: stat.AcquireDuration(),
	}
}

func buildPoolMetrics(cur, prev poolCounters, hasPrev bool) *poolMetrics {
	waitDelta := time.Duration(counterDelta(int64(cur.acquireDuration), int64(prev.acquireDuration), hasPrev))
	return &poolMetrics{
		Max:                  cur.max,
		Total:                cur.total,
		Acquired:             cur.acquired,
		Idle:                 cur.idle,
		AcquireCountDelta:    counterDelta(cur.acquireCount, prev.acquireCount, hasPrev),
		EmptyAcquireDelta:    counterDelta(cur.emptyAcquire, prev.emptyAcquire, hasPrev),
		CanceledAcquireDelta: counterDelta(cur.canceledAcquire, prev.canceledAcquire, hasPrev),
		AcquireWaitMsDelta:   float64(waitDelta) / float64(time.Millisecond),
		Utilization:          utilization(cur.acquired, cur.max),
	}
}

func utilization(acquired, maxConns int32) float64 {
	if maxConns <= 0 {
		return 0
	}
	return float64(acquired) / float64(maxConns)
}
