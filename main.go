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

	var eligibleImages []image.Summary
	for _, c := range containers.Items {
		containerImage, err := findImageForContainer(c, images.Items)
		if err != nil {
			slog.Error(err.Error())
			continue
		}
		if containerImage.RepoTags == nil {
			slog.Warn("Image %s has no repo tags - it's probably not from a registry")
			continue
		}
		if !slices.ContainsFunc(eligibleImages, func(summary image.Summary) bool {
			return summary.ID == containerImage.ID
		}) {
			eligibleImages = append(eligibleImages, containerImage)
		}
	}

	for _, eligibleImage := range eligibleImages {
		if isFromGHCR(eligibleImage) {
			slog.Info("ghcr.io image: %d", eligibleImage.RepoTags[0])
		} else {
			slog.Info("dockerhub.io image: %d", eligibleImage.RepoTags[0])
		}
	}

	slog.Info("Complete")

	os.Exit(0)
}
