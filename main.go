package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/m-milek/swiezak/internal/docker"
	"github.com/m-milek/swiezak/internal/registry"
	"github.com/m-milek/swiezak/internal/version"
)

type ImageStatus string

const (
	ImageStatusUnknown          ImageStatus = "unknown"
	ImageStatusNotCheckable     ImageStatus = "not-checkable"
	ImageStatusError            ImageStatus = "error"
	ImageStatusPinned           ImageStatus = "pinned"
	ImageStatusUpToDate         ImageStatus = "up-to-date"
	ImageStatusRebuildAvailable ImageStatus = "rebuild"
)

type CheckResult struct {
	status   ImageStatus
	upgrades [3][]string
}

func statusFromRegistryError(err error) ImageStatus {
	if registry.IsNotFound(err) {
		return ImageStatusNotCheckable
	}
	slog.Error("registry query failed", "err", err)
	return ImageStatusError
}

func main() {
	ctx := context.Background()

	images, err := docker.ListImages(ctx)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	registryClient := registry.NewClient()

	for _, img := range images {
		result := checkImage(img, registryClient)
		_, _, pin := img.Reference()
		slog.Info("result", "image", img.ConfigImage, "pinned", pin != "", "status", result.status, "major", result.upgrades[0], "minor", result.upgrades[1], "patch", result.upgrades[2])
	}

	slog.Info("Complete")
}

func checkImage(img docker.Image, registryClient *registry.Client) CheckResult {
	if len(img.RepoDigests) == 0 {
		return CheckResult{status: ImageStatusNotCheckable}
	}
	tag, hasTag, pin := img.Reference()
	if !hasTag {
		return CheckResult{status: ImageStatusPinned}
	}

	status := checkRebuild(img, tag, pin, registryClient)
	if status == ImageStatusNotCheckable || status == ImageStatusError {
		return CheckResult{status: status}
	}

	return CheckResult{
		status:   status,
		upgrades: findUpgrades(img, tag, registryClient),
	}
}

func checkRebuild(img docker.Image, tag name.Tag, pin string, registryClient *registry.Client) ImageStatus {
	remoteDigest, err := registryClient.Digest(tag.Name(), nil)
	if err != nil {
		return statusFromRegistryError(err)
	}

	upToDate := img.HasDigest(remoteDigest)
	if pin != "" {
		upToDate = remoteDigest == pin
	}
	if upToDate {
		return ImageStatusUpToDate
	}

	if img.PlatformDigest == "" {
		return ImageStatusRebuildAvailable
	}
	remotePlatformDigest, err := registryClient.Digest(tag.Name(), &img.Platform)
	if err != nil {
		slog.Error("platform digest query failed", "image", img.ConfigImage, "err", err)
		return ImageStatusRebuildAvailable
	}
	if remotePlatformDigest == img.PlatformDigest {
		return ImageStatusUpToDate
	}
	return ImageStatusRebuildAvailable
}

func findUpgrades(img docker.Image, tag name.Tag, registryClient *registry.Client) [3][]string {
	var upgrades [3][]string

	current, ok := img.Version()
	if !ok {
		return upgrades
	}
	tags, err := registryClient.Tags(tag.Context().Name())
	if err != nil {
		slog.Error("listing tags failed", "repo", tag.Context().Name(), "err", err)
		return upgrades
	}

	return version.Upgrades(current, tags)
}
