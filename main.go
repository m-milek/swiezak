package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/m-milek/swiezak/internal/check"
	"github.com/m-milek/swiezak/internal/docker"
	"github.com/m-milek/swiezak/internal/registry"
	"github.com/m-milek/swiezak/internal/report"
)

type server struct {
	keychain authn.Keychain
}

func (s server) handleDocker(w http.ResponseWriter, r *http.Request) {
	images, err := docker.ListImages(r.Context())
	if err != nil {
		slog.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	registryClient := registry.NewClient(s.keychain)
	entries := []report.Entry{}
	for _, img := range images {
		entries = append(entries, report.NewEntry(img, check.Image(img, registryClient)))
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entries); err != nil {
		slog.Error("encode response failed", "err", err)
	}
}

func main() {
	s := server{keychain: registry.KeychainFromEnv()}

	http.HandleFunc("GET /api/v1/docker", s.handleDocker)

	addr := ":1991"
	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
