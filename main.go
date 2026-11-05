package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/m-milek/swiezak/internal/docker"
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
	if terr, ok := errors.AsType[*transport.Error](err); ok && (terr.StatusCode == http.StatusUnauthorized || terr.StatusCode == http.StatusNotFound) {
		return ImageStatusNotCheckable
	}
	slog.Error("registry query failed", "err", err)
	return ImageStatusError
}

type CraneResponse struct {
	digest string
	err    error
}

func getRemoteDigest(cacheMap map[string]CraneResponse, tagName string, platform *v1.Platform) (string, error) {
	key := tagName
	var opts []crane.Option
	if platform != nil {
		key += "|" + platform.String()
		opts = append(opts, crane.WithPlatform(platform))
	}

	if cache, ok := cacheMap[key]; ok {
		return cache.digest, cache.err
	}

	remoteHash, err := crane.Digest(tagName, opts...)
	cacheMap[key] = CraneResponse{
		digest: remoteHash,
		err:    err,
	}

	return remoteHash, err
}

type TagsResponse struct {
	tags []string
	err  error
}

func getRemoteTags(cacheMap map[string]TagsResponse, repoName string) ([]string, error) {
	if cache, ok := cacheMap[repoName]; ok {
		return cache.tags, cache.err
	}

	tags, err := crane.ListTags(repoName)
	cacheMap[repoName] = TagsResponse{
		tags: tags,
		err:  err,
	}

	return tags, err
}

func main() {
	ctx := context.Background()

	images, err := docker.ListImages(ctx)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	digestCache := make(map[string]CraneResponse)
	tagsCache := make(map[string]TagsResponse)

	for _, img := range images {
		result := checkImage(img, digestCache, tagsCache)
		_, _, pin := img.Reference()
		slog.Info("result", "image", img.ConfigImage, "pinned", pin != "", "status", result.status, "major", result.upgrades[0], "minor", result.upgrades[1], "patch", result.upgrades[2])
	}

	slog.Info("Complete")
}

func checkImage(img docker.Image, digestCache map[string]CraneResponse, tagsCache map[string]TagsResponse) CheckResult {
	if len(img.RepoDigests) == 0 {
		return CheckResult{status: ImageStatusNotCheckable}
	}
	tag, hasTag, pin := img.Reference()
	if !hasTag {
		return CheckResult{status: ImageStatusPinned}
	}

	status := checkRebuild(img, tag, pin, digestCache)
	if status == ImageStatusNotCheckable || status == ImageStatusError {
		return CheckResult{status: status}
	}

	return CheckResult{
		status:   status,
		upgrades: findUpgrades(img, tag, tagsCache),
	}
}

func checkRebuild(img docker.Image, tag name.Tag, pin string, digestCache map[string]CraneResponse) ImageStatus {
	remoteDigest, err := getRemoteDigest(digestCache, tag.Name(), nil)
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
	remotePlatformDigest, err := getRemoteDigest(digestCache, tag.Name(), &img.Platform)
	if err != nil {
		slog.Error("platform digest query failed", "image", img.ConfigImage, "err", err)
		return ImageStatusRebuildAvailable
	}
	if remotePlatformDigest == img.PlatformDigest {
		return ImageStatusUpToDate
	}
	return ImageStatusRebuildAvailable
}

func findUpgrades(img docker.Image, tag name.Tag, tagsCache map[string]TagsResponse) [3][]string {
	var upgrades [3][]string

	current, ok := img.Version()
	if !ok {
		return upgrades
	}
	tags, err := getRemoteTags(tagsCache, tag.Context().Name())
	if err != nil {
		slog.Error("listing tags failed", "repo", tag.Context().Name(), "err", err)
		return upgrades
	}

	for _, candidateTag := range tags {
		candidate, err := version.ParseVersion(candidateTag)
		if err != nil {
			continue
		}
		if candidate.HasV != current.HasV ||
			candidate.Suffix != current.Suffix ||
			len(candidate.Segments) > len(current.Segments) ||
			len(strconv.Itoa(candidate.Segments[0])) > len(strconv.Itoa(current.Segments[0]))+1 {
			continue
		}
		if !version.IsCandidateVersionNewer(current, candidate) {
			continue
		}
		level := version.FirstDifferingSegment(current, candidate)
		if level < 0 || level >= len(upgrades) {
			continue
		}
		upgrades[level] = append(upgrades[level], candidateTag)
	}

	for _, list := range upgrades {
		slices.SortFunc(list, func(a, b string) int {
			aVersion, _ := version.ParseVersion(a)
			bVersion, _ := version.ParseVersion(b)
			return version.CompareVersions(bVersion, aVersion)
		})
	}
	return upgrades
}
