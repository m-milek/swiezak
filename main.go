package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	dockerclient "github.com/docker/go-sdk/client"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/moby/moby/client"
)

type ImageStatus string

const (
	ImageStatusUnknown          ImageStatus = "unknown"
	ImageStatusPinned           ImageStatus = "pinned"
	ImageStatusRebuildAvailable ImageStatus = "rebuild"
	ImageStatusUpgradeAvailable ImageStatus = "update"
)

type ImageToCheck struct {
	tag          name.Tag
	hasTag       bool
	pin          string
	imageID      string
	configImage  string
	localDigests []name.Digest
	platform     v1.Platform

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
		if localDigests == nil {
			slog.Warn("Local digests empty - skipping")
			continue
		}

		platform := v1.Platform{
			OS:           imageInspect.Os,
			Architecture: imageInspect.Architecture,
			Variant:      imageInspect.Variant,
		}

		imagesToCheck = append(imagesToCheck, ImageToCheck{
			tag:          tag,
			hasTag:       hasTag,
			pin:          pin,
			imageID:      imageInspect.ID,
			configImage:  configImage,
			localDigests: localDigests,
			platform:     platform,
			imageStatus:  ImageStatusUnknown,
		})
	}

	for i, _ := range imagesToCheck {
		entry := imagesToCheck[i]
		switch {
		case entry.hasTag && entry.pin != "":
			slog.Info(fmt.Sprintf("Image pinned with tag: %s", entry.configImage))
			remoteHash, err := crane.Digest(entry.tag.Name())
			if err != nil {
				slog.Error(err.Error())
				continue
			}
			if remoteHash != entry.pin {
				slog.Info("POSSIBLE UPDATE DETECTED")
				entry.imageStatus = ImageStatusRebuildAvailable
			}
		case entry.hasTag:
			slog.Info(fmt.Sprintf("Image not pinned: %s", entry.configImage))
			remoteHash, err := crane.Digest(entry.tag.Name())
			if err != nil {
				slog.Error(err.Error())
				continue
			}
			if !slices.ContainsFunc(entry.localDigests, func(digest name.Digest) bool {
				return digest.DigestStr() == remoteHash
			}) {
				slog.Info("POSSIBLE UPDATE DETECTED")
				entry.imageStatus = ImageStatusRebuildAvailable
			}
		default:
			slog.Info(fmt.Sprintf("Image pinned: %s", entry.configImage))
			entry.imageStatus = ImageStatusPinned
		}
	}

	slog.Info("Complete")
}
