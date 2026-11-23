// Comando obs-agent: agente de host da observabilidade (Docker + disco -> obs.*).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/obsagent"
)

func main() {
	os.Exit(run())
}

// run devolve o exit code; os.Exit fica fora para os defers rodarem.
func run() int {
	once := flag.Bool("once", false, "coleta uma vez e sai")
	dryRun := flag.Bool("dry-run", false, "imprime o JSON em vez de gravar no banco")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stderr, nil)).With("service", "obs-agent")
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	cfg := obsagent.ConfigFromEnv()

	if !cfg.Enabled && !*dryRun {
		log.Info("obs-agent: OBSERVABILITY_ENABLED=false, ocioso sem gravar")
		<-ctx.Done()
		return 0
	}

	docker, err := obsagent.NewDockerClient(obsagent.DockerHostFromEnv())
	if err != nil {
		log.Error("obs-agent: docker", "error", err.Error())
		return 1
	}
	col := &obsagent.Collector{Docker: docker, DiskPaths: obsagent.ParseDiskPaths(cfg.DiskPaths), Host: cfg.Host, Logger: log}

	if *dryRun {
		if err := dryRunOnce(ctx, col, *once, cfg); err != nil {
			log.Error("obs-agent: dry-run", "error", err.Error())
			return 1
		}
		return 0
	}

	pool, err := obsagent.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("obs-agent: banco", "error", err.Error())
		return 1
	}
	defer pool.Close()
	store := &obsagent.Store{Pool: pool}

	if *once {
		snap, err := col.Collect(ctx)
		if err == nil {
			err = store.Save(ctx, snap)
		}
		if err != nil {
			log.Error("obs-agent: ciclo unico falhou", "error", err.Error())
			return 1
		}
		return 0
	}
	log.Info("obs-agent: iniciado", "host", cfg.Host, "interval", cfg.Interval.String())
	obsagent.Run(ctx, cfg, col, store.Save, log)
	log.Info("obs-agent: encerrado")
	return 0
}

func dryRunOnce(ctx context.Context, col *obsagent.Collector, once bool, cfg obsagent.Config) error {
	print := func(ctx context.Context, snap obsagent.Snapshot) error {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(snap)
	}
	if once {
		// Duas leituras: a primeira so semeia o calculo de CPU.
		if _, err := col.Collect(ctx); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
		snap, err := col.Collect(ctx)
		if err != nil {
			return err
		}
		return print(ctx, snap)
	}
	obsagent.Run(ctx, cfg, col, print, col.Logger)
	return nil
}
