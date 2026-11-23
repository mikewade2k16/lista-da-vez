package obsagent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestCPUPercent(t *testing.T) {
	prev := CPUUsage{SystemCPUUsage: 1000}
	prev.CPUUsage.TotalUsage = 100
	cur := CPUUsage{SystemCPUUsage: 2000, OnlineCPUs: 4}
	cur.CPUUsage.TotalUsage = 200
	got, ok := CPUPercent(prev, cur)
	if !ok || math.Abs(got-40) > 1e-9 { // 100/1000*4*100
		t.Fatalf("got %v ok=%v", got, ok)
	}
	if _, ok := CPUPercent(CPUUsage{}, cur); ok {
		t.Fatal("sem leitura anterior nao deve calcular")
	}
	if _, ok := CPUPercent(cur, prev); ok {
		t.Fatal("contador regressivo nao deve calcular")
	}
}

func TestMemoryBytes(t *testing.T) {
	if got := MemoryBytes(1000, map[string]uint64{"inactive_file": 300}); got != 700 {
		t.Fatalf("v2: %d", got)
	}
	if got := MemoryBytes(1000, map[string]uint64{"total_inactive_file": 400, "cache": 900}); got != 600 {
		t.Fatalf("v1: %d", got)
	}
	if got := MemoryBytes(100, map[string]uint64{"inactive_file": 500}); got != 0 {
		t.Fatalf("clamp: %d", got)
	}
	if got := MemoryBytes(100, nil); got != 100 {
		t.Fatalf("sem stats: %d", got)
	}
}

func TestDiskFromStatfs(t *testing.T) {
	d := DiskFromStatfs("/", 1000, 250, 4096)
	if d.TotalBytes != 4096000 || d.FreeBytes != 1024000 || d.UsedPct != 75 {
		t.Fatalf("%+v", d)
	}
	if z := DiskFromStatfs("/", 0, 0, 4096); z.UsedPct != 0 {
		t.Fatalf("%+v", z)
	}
}

func TestReadDiskReal(t *testing.T) {
	d, err := ReadDisk(t.TempDir(), "/")
	if err != nil || d.Path != "/" || d.TotalBytes == 0 {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestParseDiskPaths(t *testing.T) {
	got := ParseDiskPaths(" /host=/ , /data ")
	if len(got) != 2 || got[0] != [2]string{"/host", "/"} || got[1] != [2]string{"/data", ""} {
		t.Fatalf("%v", got)
	}
	if def := ParseDiskPaths(""); len(def) != 1 || def[0][0] != "/" {
		t.Fatalf("%v", def)
	}
}

const inspectFixture = `{"Name":"/omni-api-1","RestartCount":2,
"HostConfig":{"RestartPolicy":{"Name":"unless-stopped"}},
"State":{"Status":"running","StartedAt":"2026-11-20T10:00:00.123456789Z","Health":{"Status":"healthy"}},
"Config":{"Labels":{"com.docker.compose.service":"api"}}}`

const statsFixture = `{"cpu_stats":{"cpu_usage":{"total_usage":200},"system_cpu_usage":2000,"online_cpus":2},
"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":1000,"online_cpus":2},
"memory_stats":{"usage":5000,"limit":10000,"stats":{"inactive_file":1000}}}`

func fakeDocker(t *testing.T) *DockerClient {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "obs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "d.sock")
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", sock)
	if err != nil {
		t.Skip("socket unix indisponivel:", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/containers/json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Query().Get("all") != "1" {
			http.Error(w, "x", 400)
			return
		}
		io.WriteString(w, `[{"Id":"abc","Names":["/omni-api-1"],"State":"running"},{"Id":"def","Names":["/job"],"State":"exited"}]`)
	})
	mux.HandleFunc("/containers/abc/json", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, inspectFixture) })
	mux.HandleFunc("/containers/def/json", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"Name":"/job","HostConfig":{"RestartPolicy":{"Name":"no"}},"State":{"Status":"exited","ExitCode":137,"StartedAt":"0001-01-01T00:00:00Z"},"Config":{"Labels":{}}}`)
	})
	mux.HandleFunc("/containers/abc/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("stream") != "false" {
			http.Error(w, "stream", 400)
			return
		}
		io.WriteString(w, statsFixture)
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return newDockerClientForSocket(sock)
}

func TestCollectWithFakeDocker(t *testing.T) {
	col := &Collector{
		Docker:    fakeDocker(t),
		DiskPaths: [][2]string{{t.TempDir(), "/"}, {"/nao/existe", ""}},
		Host:      "h1",
		Logger:    slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	snap, err := col.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.System.Docker.Containers != 2 || snap.System.Docker.Running != 1 || len(snap.System.Disk) != 1 {
		t.Fatalf("%+v", snap.System)
	}
	byName := map[string]ContainerSample{}
	for _, c := range snap.Containers {
		byName[c.Container] = c
	}
	api := byName["omni-api-1"]
	if api.Service != "api" || api.Health != "healthy" || api.RestartCount != 2 || api.RestartPolicy != "unless-stopped" || api.ExitCode != nil || api.StartedAt == nil {
		t.Fatalf("%+v", api)
	}
	if api.CPUPct == nil || *api.CPUPct != 20 || *api.MemBytes != 4000 || *api.MemLimitBytes != 10000 {
		t.Fatalf("%+v", api)
	}
	job := byName["job"]
	if job.State != "exited" || job.RestartPolicy != "no" || job.ExitCode == nil || *job.ExitCode != 137 || job.StartedAt != nil || job.CPUPct != nil {
		t.Fatalf("%+v", job)
	}
	raw, _ := json.Marshal(snap.System)
	var back map[string]any
	_ = json.Unmarshal(raw, &back)
	if _, ok := back["disk"]; !ok {
		t.Fatalf("json: %s", raw)
	}
}

func TestNewDockerClientRejectsTCP(t *testing.T) {
	if _, err := NewDockerClient("tcp://1.2.3.4:2375"); err == nil {
		t.Fatal("esperava erro")
	}
}
