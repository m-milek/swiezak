package main

import (
	"cmp"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/m-milek/swiezak/internal/cache"
	"github.com/m-milek/swiezak/internal/registry"
	"github.com/m-milek/swiezak/internal/report"
)

const refreshInterval = 8 * time.Hour

func main() {
	keychain := registry.KeychainFromEnv()
	reportPath := cmp.Or(os.Getenv("REPORT_PATH"), "report.json")

	reportCache := cache.New(reportPath, refreshInterval, func(ctx context.Context) ([]byte, error) {
		entries, err := report.Build(ctx, keychain)
		if err != nil {
			return nil, err
		}
		return json.Marshal(entries)
	})
	go reportCache.Run(context.Background())

	http.HandleFunc("GET /api/v1/docker", func(w http.ResponseWriter, r *http.Request) {
		data := reportCache.Get()
		if data == nil {
			http.Error(w, "report not ready yet", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	addr := ":1991"
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
