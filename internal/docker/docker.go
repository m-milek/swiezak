package docker

import (
	"context"
	"fmt"
	"log/slog"

	dockerclient "github.com/docker/go-sdk/client"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/moby/moby/client"
)

func normalizePlatform(p v1.Platform) v1.Platform {
	if p.Architecture == "arm64" && p.Variant == "v8" {
		p.Variant = ""
	}
	return p
}

func ListImages(ctx context.Context) ([]Image, error) {
	dockerClient, err := dockerclient.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to docker: %w", err)
	}
	defer dockerClient.Close()

	containers, err := dockerClient.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	slog.Info(fmt.Sprintf("Containers: %d", len(containers.Items)))

	var images []Image
	for _, c := range containers.Items {
		containerInspect, err := dockerClient.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
		if err != nil {
			slog.Error(err.Error())
			continue
		}
		imageInspect, err := dockerClient.ImageInspect(ctx, c.ImageID)
		if err != nil {
			slog.Error(err.Error())
			continue
		}
		configImage := containerInspect.Container.Config.Image

		if _, _, _, err := SplitReference(configImage); err != nil {
			slog.Error(err.Error())
			continue
		}

		platform := v1.Platform{
			OS:           imageInspect.Os,
			Architecture: imageInspect.Architecture,
			Variant:      imageInspect.Variant,
		}
		platformDigest := ""
		if d := containerInspect.Container.ImageManifestDescriptor; d != nil {
			platformDigest = d.Digest.String()
			if d.Platform != nil {
				platform = v1.Platform{
					OS:           d.Platform.OS,
					Architecture: d.Platform.Architecture,
					Variant:      d.Platform.Variant,
				}
			}
		}

		images = append(images, Image{
			ConfigImage:    configImage,
			RepoDigests:    imageInspect.RepoDigests,
			Platform:       normalizePlatform(platform),
			PlatformDigest: platformDigest,
		})
	}

	return images, nil
}
