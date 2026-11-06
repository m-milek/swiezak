package check

import (
	"log/slog"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/m-milek/swiezak/internal/docker"
	"github.com/m-milek/swiezak/internal/registry"
	"github.com/m-milek/swiezak/internal/version"
)

type Status string

const (
	StatusNotCheckable     Status = "not-checkable"
	StatusError            Status = "error"
	StatusPinned           Status = "pinned"
	StatusUpToDate         Status = "up-to-date"
	StatusRebuildAvailable Status = "rebuild"
)

type Result struct {
	Status Status
	Major  []string
	Minor  []string
	Patch  []string
	Error  string
}

func Image(img docker.Image, registryClient *registry.Client) Result {
	if len(img.RepoDigests) == 0 {
		return Result{Status: StatusNotCheckable}
	}
	tag, hasTag, pin := img.Reference()
	if !hasTag {
		return Result{Status: StatusPinned}
	}

	status, err := checkRebuild(img, tag, pin, registryClient)
	if status == StatusError {
		return Result{Status: status, Error: err.Error()}
	}
	if status == StatusNotCheckable {
		return Result{Status: status}
	}

	upgrades := findUpgrades(img, tag, registryClient)
	return Result{
		Status: status,
		Major:  upgrades[0],
		Minor:  upgrades[1],
		Patch:  upgrades[2],
	}
}

func statusFromRegistryError(err error) Status {
	if registry.IsNotFound(err) {
		return StatusNotCheckable
	}
	slog.Error("registry query failed", "err", err)
	return StatusError
}

func checkRebuild(img docker.Image, tag name.Tag, pin string, registryClient *registry.Client) (Status, error) {
	remoteDigest, err := registryClient.Digest(tag.Name(), nil)
	if err != nil {
		return statusFromRegistryError(err), err
	}

	upToDate := img.HasDigest(remoteDigest)
	if pin != "" {
		upToDate = remoteDigest == pin
	}
	if upToDate {
		return StatusUpToDate, nil
	}

	if img.PlatformDigest == "" {
		return StatusRebuildAvailable, nil
	}
	remotePlatformDigest, err := registryClient.Digest(tag.Name(), &img.Platform)
	if err != nil {
		slog.Error("platform digest query failed", "image", img.ConfigImage, "err", err)
		return StatusRebuildAvailable, nil
	}
	if remotePlatformDigest == img.PlatformDigest {
		return StatusUpToDate, nil
	}
	return StatusRebuildAvailable, nil
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
