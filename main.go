package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	dockerclient "github.com/docker/go-sdk/client"
	"github.com/moby/moby/client"
)

func main() {
	slog.Info("hello")
	ctx := context.Background()

	docker, err := dockerclient.New(ctx)
	if err != nil {
		slog.Error("Failed to connect to Docker socket")
		os.Exit(1)
	}
	defer docker.Close()

	containers, err := docker.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	images, err := docker.ImageList(ctx, client.ImageListOptions{All: true})
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	slog.Info(fmt.Sprintf("Containers: %d, Images: %d", len(containers.Items), len(images.Items)))

	os.Exit(0)
}
