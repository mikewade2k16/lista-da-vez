package obsagent

import (
	"strings"
	"syscall"
	"time"
)

// CPUPercent aplica o calculo do `docker stats` entre duas leituras. Devolve
// ok=false quando nao ha janela valida (primeira leitura, container recem-criado).
func CPUPercent(prev, cur CPUUsage) (float64, bool) {
	if prev.SystemCPUUsage == 0 || cur.SystemCPUUsage <= prev.SystemCPUUsage || cur.CPUUsage.TotalUsage < prev.CPUUsage.TotalUsage {
		return 0, false
	}
	cpuDelta := float64(cur.CPUUsage.TotalUsage - prev.CPUUsage.TotalUsage)
	sysDelta := float64(cur.SystemCPUUsage - prev.SystemCPUUsage)
	online := float64(cur.OnlineCPUs)
	if online == 0 {
		online = float64(len(cur.CPUUsage.PercpuUsage))
	}
	if online == 0 {
		online = 1
	}
	return cpuDelta / sysDelta * online * 100, true
}

// MemoryBytes desconta o cache de pagina como o `docker stats` (cgroup v1:
// total_inactive_file; v2: inactive_file; fallback cache). Nunca negativo.
func MemoryBytes(usage uint64, stats map[string]uint64) uint64 {
	for _, key := range []string{"total_inactive_file", "inactive_file", "cache"} {
		if v, ok := stats[key]; ok {
			if v < usage {
				return usage - v
			}
			return 0
		}
	}
	return usage
}

// DiskUsage e uma medicao de um ponto de montagem.
type DiskUsage struct {
	Path       string  `json:"path"`
	TotalBytes uint64  `json:"totalBytes"`
	FreeBytes  uint64  `json:"freeBytes"`
	UsedPct    float64 `json:"usedPct"`
}

// DiskFromStatfs converte blocos do statfs em bytes; Bavail e o espaco livre
// para usuarios nao privilegiados (mesma base do `df`).
func DiskFromStatfs(label string, blocks, bavail uint64, bsize uint64) DiskUsage {
	total := blocks * bsize
	free := bavail * bsize
	used := 0.0
	if total > 0 {
		used = float64(total-minU64(free, total)) / float64(total) * 100
	}
	return DiskUsage{Path: label, TotalBytes: total, FreeBytes: free, UsedPct: round2(used)}
}

// ReadDisk mede o caminho; label e o nome gravado (vazio = o proprio caminho).
func ReadDisk(path, label string) (DiskUsage, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return DiskUsage{}, err
	}
	if label == "" {
		label = path
	}
	return DiskFromStatfs(label, uint64(st.Blocks), uint64(st.Bavail), uint64(st.Bsize)), nil //nolint:unconvert // tipos variam por plataforma
}

// ParseDiskPaths le "caminho" ou "caminho=rotulo" separados por virgula.
func ParseDiskPaths(raw string) [][2]string {
	var out [][2]string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		path, label, _ := strings.Cut(part, "=")
		out = append(out, [2]string{strings.TrimSpace(path), strings.TrimSpace(label)})
	}
	if len(out) == 0 {
		out = append(out, [2]string{"/", ""})
	}
	return out
}

func minU64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// ParseDockerTime le StartedAt do Docker; "0001-01-01..." e vazio viram nil.
func ParseDockerTime(raw string) *time.Time {
	if raw == "" || strings.HasPrefix(raw, "0001-01-01") {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil
	}
	return &t
}
