package docker

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/m-milek/swiezak/internal/version"
)

type Image struct {
	ConfigImage    string
	RepoDigests    []string
	Platform       v1.Platform
	PlatformDigest string
}

func (i Image) Reference() (tag name.Tag, hasTag bool, pin string) {
	tag, hasTag, pin, _ = SplitReference(i.ConfigImage)
	return tag, hasTag, pin
}

func (i Image) Version() (version.TagVersion, bool) {
	tag, hasTag, _ := i.Reference()
	if !hasTag {
		return version.TagVersion{}, false
	}
	v, err := version.ParseVersion(tag.TagStr())
	return v, err == nil
}

func (i Image) HasDigest(digest string) bool {
	return slices.ContainsFunc(i.RepoDigests, func(repoDigest string) bool {
		_, d, ok := strings.Cut(repoDigest, "@")
		return ok && d == digest
	})
}

func SplitReference(image string) (tag name.Tag, hasTag bool, pin string, err error) {
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
