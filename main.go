package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/m-milek/swiezak/internal/check"
	"github.com/m-milek/swiezak/internal/docker"
	"github.com/m-milek/swiezak/internal/registry"
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
		result := check.Image(img, registryClient)
		_, _, pin := img.Reference()
		slog.Info("result", "image", img.ConfigImage, "pinned", pin != "", "status", result.Status, "major", result.Major, "minor", result.Minor, "patch", result.Patch)
	}

	slog.Info("Complete")
}
