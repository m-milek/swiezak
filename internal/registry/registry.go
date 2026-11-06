package registry

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

type digestResponse struct {
	digest string
	err    error
}

type manifestResponse struct {
	desc *remote.Descriptor
	err  error
}

type tagsResponse struct {
	tags []string
	err  error
}

type Client struct {
	opts      []crane.Option
	digests   map[string]digestResponse
	manifests map[string]manifestResponse
	tags      map[string]tagsResponse
}

func NewClient(keychain authn.Keychain) *Client {
	return &Client{
		opts:      []crane.Option{crane.WithAuthFromKeychain(keychain)},
		digests:   make(map[string]digestResponse),
		manifests: make(map[string]manifestResponse),
		tags:      make(map[string]tagsResponse),
	}
}

func (c *Client) Digest(tagName string, platform *v1.Platform) (string, error) {
	if platform != nil {
		return c.platformDigest(tagName, *platform)
	}

	if cached, ok := c.digests[tagName]; ok {
		return cached.digest, cached.err
	}

	digest, err := crane.Digest(tagName, c.opts...)
	c.digests[tagName] = digestResponse{digest: digest, err: err}
	return digest, err
}

func (c *Client) manifest(tagName string) (*remote.Descriptor, error) {
	if cached, ok := c.manifests[tagName]; ok {
		return cached.desc, cached.err
	}

	desc, err := crane.Get(tagName, c.opts...)
	c.manifests[tagName] = manifestResponse{desc: desc, err: err}
	return desc, err
}

func (c *Client) platformDigest(tagName string, platform v1.Platform) (string, error) {
	desc, err := c.manifest(tagName)
	if err != nil {
		return "", err
	}
	if !desc.MediaType.IsIndex() {
		return desc.Digest.String(), nil
	}

	index, err := v1.ParseIndexManifest(bytes.NewReader(desc.Manifest))
	if err != nil {
		return "", fmt.Errorf("parse index %s: %w", tagName, err)
	}
	for _, m := range index.Manifests {
		if m.Platform != nil && m.Platform.Satisfies(platform) {
			return m.Digest.String(), nil
		}
	}
	return "", fmt.Errorf("no manifest for platform %s in %s", platform.String(), tagName)
}

func (c *Client) Tags(repoName string) ([]string, error) {
	if cached, ok := c.tags[repoName]; ok {
		return cached.tags, cached.err
	}

	tags, err := crane.ListTags(repoName, c.opts...)
	c.tags[repoName] = tagsResponse{tags: tags, err: err}
	return tags, err
}

func IsNotFound(err error) bool {
	terr, ok := errors.AsType[*transport.Error](err)
	return ok && (terr.StatusCode == http.StatusUnauthorized || terr.StatusCode == http.StatusNotFound)
}
