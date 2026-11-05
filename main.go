package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/m-milek/swiezak/internal/check"
	"github.com/m-milek/swiezak/internal/docker"
	"github.com/m-milek/swiezak/internal/registry"
	"github.com/m-milek/swiezak/internal/report"
)

func main() {
	ctx := context.Background()

	images, err := docker.ListImages(ctx)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	registryClient := registry.NewClient()

	for _, img := range images {
		report.Print(img, check.Image(img, registryClient))
	}

	slog.Info("Complete")
}
