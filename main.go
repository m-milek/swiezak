package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"

	dockerclient "github.com/docker/go-sdk/client"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
)

type Registry string

const (
	RegistryDockerHub Registry = "Docker Hub"
	RegistryGHCR      Registry = "GitHub Container Registry"
)

type ImageStatus string

const (
	ImageStatusUnknown          ImageStatus = "unknown"
	ImageStatusPinned           ImageStatus = "pinned"
	ImageStatusRebuildAvailable ImageStatus = "rebuild"
	ImageStatusUpgradeAvailable ImageStatus = "update"
)

func isFromGHCR(image image.Summary) bool {
	return slices.ContainsFunc(image.RepoTags, func(s string) bool {
		return strings.HasPrefix(s, "ghcr.io/")
	})
}

func isPinned(image image.Summary) bool {
	pattern := regexp.MustCompile(`.*/@sha256:`)
	return slices.ContainsFunc(image.RepoTags, func(s string) bool {
		return pattern.MatchString(s)
	})
}

func findImageForContainer(container container.Summary, images []image.Summary) (image.Summary, error) {
	imageIndex := slices.IndexFunc(images, func(summary image.Summary) bool {
		return summary.ID == container.ImageID
	})
	if imageIndex == -1 {
		return image.Summary{}, fmt.Errorf("no image found for container %s", container.ID)
	}
	return images[imageIndex], nil
}

func main() {
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

	var eligibleRefs []name.Reference
	for _, c := range containers.Items {
		containerInspect, err := docker.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
		if err != nil {
			slog.Error(err.Error())
			continue
		}
		imageRef := containerInspect.Container.Config.Image
		reference, err := name.ParseReference(imageRef)
		if err != nil {
			slog.Error(err.Error())
			continue
		}
		if !slices.ContainsFunc(eligibleRefs, func(r name.Reference) bool {
			return r == reference
		}) {
			eligibleRefs = append(eligibleRefs, reference)
		}
	}

	for _, ref := range eligibleRefs {

	}

	slog.Info("Complete")

	os.Exit(0)
}
