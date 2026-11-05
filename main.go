package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"

	dockerclient "github.com/docker/go-sdk/client"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/moby/moby/client"
)

type ImageStatus string

const (
	ImageStatusUnknown          ImageStatus = "unknown"
	ImageStatusNotCheckable     ImageStatus = "not-checkable"
	ImageStatusError            ImageStatus = "error"
	ImageStatusPinned           ImageStatus = "pinned"
	ImageStatusUpToDate         ImageStatus = "up-to-date"
	ImageStatusRebuildAvailable ImageStatus = "rebuild"
	ImageStatusUpgradeAvailable ImageStatus = "upgrade"
)

type ImageToCheck struct {
	tag                          name.Tag
	hasTag                       bool
	pin                          string
	imageID                      string
	configImage                  string
	localDigests                 []name.Digest
	localPlatformManifestDigest  string
	remotePlatformManifestDigest string
	platform                     v1.Platform

	imageStatus ImageStatus
}

func splitReference(image string) (tag name.Tag, hasTag bool, pin string, err error) {
	beforePin, pin, hasPin := strings.Cut(image, "@")

	lastSegment := beforePin
	if _, after, found := strings.CutLast(beforePin, "/"); found {
		lastSegment = after
	}

	if hasPin && !strings.Contains(lastSegment, ":") {
		return name.Tag{}, false, pin, nil
	}

	tag, err = name.NewTag(beforePin)
	if err != nil {
		return name.Tag{}, false, "", fmt.Errorf("parse tag %q: %w", beforePin, err)
	}
	return tag, true, pin, nil
}

func statusFromRegistryError(err error) ImageStatus {
	if terr, ok := errors.AsType[*transport.Error](err); ok && (terr.StatusCode == http.StatusUnauthorized || terr.StatusCode == http.StatusNotFound) {
		return ImageStatusNotCheckable
	}
	slog.Error("registry query failed", "err", err)
	return ImageStatusError
}

func normalizePlatform(p v1.Platform) v1.Platform {
	if p.Architecture == "arm64" && p.Variant == "v8" {
		p.Variant = ""
	}
	return p
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

	slog.Info(fmt.Sprintf("Containers: %d", len(containers.Items)))

	var imagesToCheck []ImageToCheck
	for _, c := range containers.Items {
		containerInspect, err := docker.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
		if err != nil {
			slog.Error(err.Error())
			continue
		}
		imageInspect, err := docker.ImageInspect(ctx, c.ImageID)
		if err != nil {
			slog.Error(err.Error())
			continue
		}
		configImage := containerInspect.Container.Config.Image

		tag, hasTag, pin, err := splitReference(configImage)
		if err != nil {
			slog.Error(err.Error())
			continue
		}

		var localDigests []name.Digest
		for _, digestStr := range imageInspect.RepoDigests {
			digest, err := name.NewDigest(digestStr)
			if err != nil {
				slog.Error(err.Error())
				continue
			}
			localDigests = append(localDigests, digest)
		}
		status := ImageStatusUnknown
		if localDigests == nil {
			slog.Warn("Local digests empty", "image", configImage)
			status = ImageStatusNotCheckable
		}

		platform := v1.Platform{
			OS:           imageInspect.Os,
			Architecture: imageInspect.Architecture,
			Variant:      imageInspect.Variant,
		}
		if d := containerInspect.Container.ImageManifestDescriptor; d != nil && d.Platform != nil {
			platform = v1.Platform{
				OS:           d.Platform.OS,
				Architecture: d.Platform.Architecture,
				Variant:      d.Platform.Variant,
			}
		}
		platform = normalizePlatform(platform)

		var localPlatformManifestDigest = ""
		if containerInspect.Container.ImageManifestDescriptor != nil {
			localPlatformManifestDigest = containerInspect.Container.ImageManifestDescriptor.Digest.String()
		}

		imagesToCheck = append(imagesToCheck, ImageToCheck{
			tag:                         tag,
			hasTag:                      hasTag,
			pin:                         pin,
			imageID:                     imageInspect.ID,
			configImage:                 configImage,
			localDigests:                localDigests,
			localPlatformManifestDigest: localPlatformManifestDigest,
			platform:                    platform,
			imageStatus:                 status,
		})
	}

	var craneCache = make(map[string]CraneResponse)

	for i := range imagesToCheck {
		entry := &imagesToCheck[i]
		if entry.imageStatus == ImageStatusNotCheckable {
			continue
		}
		if !entry.hasTag {
			entry.imageStatus = ImageStatusPinned
			continue
		}
		remoteHash, err := getRemoteDigest(craneCache, entry.tag.Name(), nil)
		if err != nil {
			entry.imageStatus = statusFromRegistryError(err)
			continue
		}
		switch {
		case entry.hasTag && entry.pin != "":
			if remoteHash != entry.pin {
				entry.imageStatus = ImageStatusRebuildAvailable
			} else {
				entry.imageStatus = ImageStatusUpToDate
			}
		case entry.hasTag:
			if slices.ContainsFunc(entry.localDigests, func(digest name.Digest) bool {
				return digest.DigestStr() == remoteHash
			}) {
				entry.imageStatus = ImageStatusUpToDate
			} else {
				entry.imageStatus = ImageStatusRebuildAvailable
			}
		}

		if entry.imageStatus == ImageStatusRebuildAvailable {
			remotePlatformHash, err := getRemoteDigest(craneCache, entry.tag.Name(), &entry.platform)
			if err != nil {
				entry.imageStatus = statusFromRegistryError(err)
				continue
			}
			entry.remotePlatformManifestDigest = remotePlatformHash
			if entry.localPlatformManifestDigest != "" && entry.remotePlatformManifestDigest == entry.localPlatformManifestDigest {
				entry.imageStatus = ImageStatusUpToDate
			}
		}
	}

	for _, entry := range imagesToCheck {
		slog.Info("result", "image", entry.configImage, "pinned", entry.pin != "", "status", entry.imageStatus, "platform", entry.platform.String(), "local", entry.localPlatformManifestDigest, "remote", entry.remotePlatformManifestDigest)
	}

	slog.Info("Complete")
}
