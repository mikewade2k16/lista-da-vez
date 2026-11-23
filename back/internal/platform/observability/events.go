package observability

import (
	"math"
	"time"
)

type httpEvent struct {
	OccurredAt    time.Time
	CorrelationID string
	Method        string
	Route         string
	PathModule    string
	Status        int
	DurationMS    float64
	ResponseBytes int64
	UserID        string
	AccountID     string
	QueryCount    int
	QueryMS       float64
	ExternalCount int
	ErrorCode     string
	ErrorMessage  string
	ErrorStack    string
	Source        string
}

type slowQueryEvent struct {
	OccurredAt    time.Time
	CorrelationID string
	Route         string
	Fingerprint   string
	Statement     string
	DurationMS    float64
	Error         string
}

type nPlusOneEvent struct {
	OccurredAt    time.Time
	CorrelationID string
	Route         string
	Fingerprint   string
	Statement     string
	Repetitions   int
}

type externalEvent struct {
	OccurredAt    time.Time
	CorrelationID string
	Route         string
	Integration   string
	Method        string
	Path          string
	Status        int
	DurationMS    float64
	Timeout       bool
	Error         string
}

type frontendEvent struct {
	OccurredAt    time.Time
	PageViewID    string
	Page          string
	Kind          string
	Name          string
	Value         *float64
	Status        *int
	CorrelationID string
	Detail        map[string]any
	UserID        string
	UserAgent     string
}

// LatencyBucketsMS são os limites superiores (ms) do histograma por rota. O último
// bucket é aberto. Percentis do painel são interpolados a partir deles, o que
// mantém contagens exatas mesmo com amostragem dos dados brutos.
var LatencyBucketsMS = []float64{5, 10, 25, 50, 75, 100, 150, 200, 300, 500, 750, 1000, 1500, 2000, 3000, 5000, 10000, 30000}

func bucketIndex(durationMS float64) int {
	for index, limit := range LatencyBucketsMS {
		if durationMS <= limit {
			return index
		}
	}
	return len(LatencyBucketsMS)
}

// PercentileFromBuckets estima o percentil p (0–1) a partir das contagens.
func PercentileFromBuckets(buckets []int64, p float64) float64 {
	var total int64
	for _, count := range buckets {
		total += count
	}
	if total == 0 {
		return 0
	}
	target := p * float64(total)
	var cumulative float64
	for index, count := range buckets {
		if count == 0 {
			continue
		}
		lower := 0.0
		if index > 0 {
			lower = LatencyBucketsMS[min(index-1, len(LatencyBucketsMS)-1)]
		}
		upper := lower * 2
		if index < len(LatencyBucketsMS) {
			upper = LatencyBucketsMS[index]
		}
		if cumulative+float64(count) >= target {
			fraction := (target - cumulative) / float64(count)
			return math.Round((lower+fraction*(upper-lower))*10) / 10
		}
		cumulative += float64(count)
	}
	return LatencyBucketsMS[len(LatencyBucketsMS)-1]
}

type routeKey struct {
	Bucket time.Time
	Route  string
}

type routeAggregate struct {
	PathModule   string
	Calls        int64
	Errors4xx    int64
	Errors5xx    int64
	TotalMS      float64
	MaxMS        float64
	QueriesTotal int64
	QueriesMax   int
	Buckets      []int64
}

type queryKey struct {
	Bucket      time.Time
	Fingerprint string
	Route       string
}

type queryAggregate struct {
	Statement string
	Verb      string
	Calls     int64
	Errors    int64
	TotalMS   float64
	MaxMS     float64
}
