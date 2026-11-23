package obsagent

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ContainerSample espelha uma linha de obs.container_samples.
type ContainerSample struct {
	Container     string     `json:"container"`
	Service       string     `json:"service"`
	State         string     `json:"state"`
	Health        string     `json:"health"`
	RestartCount  int        `json:"restartCount"`
	RestartPolicy string     `json:"restartPolicy"`
	ExitCode      *int       `json:"exitCode"`
	StartedAt     *time.Time `json:"startedAt"`
	CPUPct        *float64   `json:"cpuPct"`
	MemBytes      *int64     `json:"memBytes"`
	MemLimitBytes *int64     `json:"memLimitBytes"`
}

// Snapshot e o resultado de um ciclo de coleta.
type Snapshot struct {
	At         time.Time         `json:"at"`
	Host       string            `json:"host"`
	Containers []ContainerSample `json:"containers"`
	System     SystemMetrics     `json:"system"`
}

// SystemMetrics e o jsonb de obs.system_samples (source='agent').
type SystemMetrics struct {
	Disk   []DiskUsage `json:"disk"`
	Docker struct {
		Containers int `json:"containers"`
		Running    int `json:"running"`
	} `json:"docker"`
}

// DockerAPI e o que o coletor precisa do cliente Docker (facilita teste).
type DockerAPI interface {
	ListContainers(ctx context.Context) ([]ContainerSummary, error)
	InspectContainer(ctx context.Context, id string) (ContainerInspect, error)
	ContainerStats(ctx context.Context, id string) (ContainerStats, error)
}

// Collector coleta Docker e disco. Guarda a ultima leitura de CPU por container
// porque o one-shot do Docker nao traz precpu_stats confiavel.
type Collector struct {
	Docker    DockerAPI
	DiskPaths [][2]string
	Host      string
	Logger    *slog.Logger

	mu      sync.Mutex
	prevCPU map[string]CPUUsage
}

const statsParallelism = 4

// Collect nunca falha por causa de um container: o erro vira log e campos nulos.
// So devolve erro se a listagem de containers falhar.
func (c *Collector) Collect(ctx context.Context) (Snapshot, error) {
	snap := Snapshot{At: time.Now().UTC(), Host: c.Host, Containers: []ContainerSample{}}
	snap.System.Disk = []DiskUsage{}

	for _, p := range c.DiskPaths {
		d, err := ReadDisk(p[0], p[1])
		if err != nil {
			c.Logger.Warn("obs-agent: falha ao medir disco", "path", p[0], "error", err.Error())
			continue
		}
		snap.System.Disk = append(snap.System.Disk, d)
	}

	list, err := c.Docker.ListContainers(ctx)
	if err != nil {
		return snap, err
	}

	samples := make([]ContainerSample, len(list))
	valid := make([]bool, len(list))
	sem := make(chan struct{}, statsParallelism)
	var wg sync.WaitGroup
	seen := make(map[string]CPUUsage, len(list))
	var seenMu sync.Mutex
	for i, item := range list {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					c.Logger.Error("obs-agent: panic ao coletar container", "container", firstName(item), "panic", r)
				}
			}()
			s, cur, ok := c.collectOne(ctx, item)
			samples[i], valid[i] = s, ok
			if ok && cur != nil {
				seenMu.Lock()
				seen[item.ID] = *cur
				seenMu.Unlock()
			}
		}()
	}
	wg.Wait()

	c.mu.Lock()
	c.prevCPU = seen
	c.mu.Unlock()

	for i, s := range samples {
		if !valid[i] {
			continue
		}
		snap.Containers = append(snap.Containers, s)
		if s.State == "running" {
			snap.System.Docker.Running++
		}
	}
	snap.System.Docker.Containers = len(snap.Containers)
	return snap, nil
}

func (c *Collector) collectOne(ctx context.Context, item ContainerSummary) (ContainerSample, *CPUUsage, bool) {
	name := firstName(item)
	insp, err := c.Docker.InspectContainer(ctx, item.ID)
	if err != nil {
		c.Logger.Warn("obs-agent: falha no inspect", "container", name, "error", err.Error())
		return ContainerSample{}, nil, false
	}
	if n := strings.TrimPrefix(insp.Name, "/"); n != "" {
		name = n
	}
	s := ContainerSample{
		Container:     name,
		Service:       insp.Config.Labels["com.docker.compose.service"],
		State:         insp.State.Status,
		RestartCount:  insp.RestartCount,
		RestartPolicy: insp.HostConfig.RestartPolicy.Name,
		StartedAt:     ParseDockerTime(insp.State.StartedAt),
	}
	if insp.State.Health != nil {
		s.Health = insp.State.Health.Status
	}
	if s.State != "running" {
		code := insp.State.ExitCode
		s.ExitCode = &code
		return s, nil, true
	}
	st, err := c.Docker.ContainerStats(ctx, item.ID)
	if err != nil {
		c.Logger.Warn("obs-agent: falha nos stats", "container", name, "error", err.Error())
		return s, nil, true
	}
	prev := st.PreCPUStats
	if prev.SystemCPUUsage == 0 {
		c.mu.Lock()
		prev = c.prevCPU[item.ID]
		c.mu.Unlock()
	}
	if pct, ok := CPUPercent(prev, st.CPUStats); ok {
		pct = round2(pct)
		s.CPUPct = &pct
	}
	mem := int64(MemoryBytes(st.MemoryStats.Usage, st.MemoryStats.Stats))
	s.MemBytes = &mem
	if st.MemoryStats.Limit > 0 {
		lim := int64(st.MemoryStats.Limit)
		s.MemLimitBytes = &lim
	}
	cur := st.CPUStats
	return s, &cur, true
}

func firstName(c ContainerSummary) string {
	if len(c.Names) > 0 {
		return strings.TrimPrefix(c.Names[0], "/")
	}
	if len(c.ID) > 12 {
		return c.ID[:12]
	}
	return c.ID
}
