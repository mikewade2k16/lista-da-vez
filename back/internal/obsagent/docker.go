package obsagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultDockerSocket = "/var/run/docker.sock"

// DockerClient fala com a Docker Engine API pelo socket unix. Somente leitura:
// o unico metodo HTTP emitido e GET (get nao aceita outro verbo).
type DockerClient struct {
	http *http.Client
}

// NewDockerClient resolve o socket de DOCKER_HOST (unix://...) ou do caminho padrao.
func NewDockerClient(dockerHost string) (*DockerClient, error) {
	socket := defaultDockerSocket
	host := strings.TrimSpace(dockerHost)
	if host != "" {
		if !strings.HasPrefix(host, "unix://") {
			return nil, fmt.Errorf("DOCKER_HOST %q nao suportado: somente unix://", host)
		}
		socket = strings.TrimPrefix(host, "unix://")
	}
	return newDockerClientForSocket(socket), nil
}

func newDockerClientForSocket(socket string) *DockerClient {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socket)
		},
		MaxIdleConns:    4,
		IdleConnTimeout: 30 * time.Second,
	}
	return &DockerClient{http: &http.Client{Transport: transport}}
}

// DockerHostFromEnv devolve DOCKER_HOST sem interpretar.
func DockerHostFromEnv() string { return os.Getenv("DOCKER_HOST") }

func (c *DockerClient) get(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := "http://docker" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("docker %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
}

// ListContainers lista todos os containers (inclusive parados).
func (c *DockerClient) ListContainers(ctx context.Context) ([]ContainerSummary, error) {
	var out []ContainerSummary
	err := c.get(ctx, "/containers/json", url.Values{"all": {"1"}}, &out)
	return out, err
}

// InspectContainer le o estado detalhado de um container.
func (c *DockerClient) InspectContainer(ctx context.Context, id string) (ContainerInspect, error) {
	var out ContainerInspect
	err := c.get(ctx, "/containers/"+url.PathEscape(id)+"/json", nil, &out)
	return out, err
}

// ContainerStats le uma amostra unica de recursos.
func (c *DockerClient) ContainerStats(ctx context.Context, id string) (ContainerStats, error) {
	var out ContainerStats
	err := c.get(ctx, "/containers/"+url.PathEscape(id)+"/stats", url.Values{"stream": {"false"}, "one-shot": {"true"}}, &out)
	return out, err
}

// ContainerSummary e o recorte usado de GET /containers/json.
type ContainerSummary struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	State string   `json:"State"`
}

// ContainerInspect e o recorte usado de GET /containers/{id}/json.
type ContainerInspect struct {
	Name         string `json:"Name"`
	RestartCount int    `json:"RestartCount"`
	HostConfig   struct {
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
	} `json:"HostConfig"`
	State struct {
		Status    string `json:"Status"`
		ExitCode  int    `json:"ExitCode"`
		StartedAt string `json:"StartedAt"`
		Health    *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

// CPUUsage e o bloco cpu_stats/precpu_stats.
type CPUUsage struct {
	CPUUsage struct {
		TotalUsage  uint64   `json:"total_usage"`
		PercpuUsage []uint64 `json:"percpu_usage"`
	} `json:"cpu_usage"`
	SystemCPUUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs     uint32 `json:"online_cpus"`
}

// ContainerStats e o recorte usado de GET /containers/{id}/stats.
type ContainerStats struct {
	CPUStats    CPUUsage `json:"cpu_stats"`
	PreCPUStats CPUUsage `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
}
