package registry

import (
	"errors"
	"net/http"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

type digestResponse struct {
	digest string
	err    error
}

type tagsResponse struct {
	tags []string
	err  error
}

type Client struct {
	digests map[string]digestResponse
	tags    map[string]tagsResponse
}

func NewClient() *Client {
	return &Client{
		digests: make(map[string]digestResponse),
		tags:    make(map[string]tagsResponse),
	}
}

func (c *Client) Digest(tagName string, platform *v1.Platform) (string, error) {
	key := tagName
	var opts []crane.Option
	if platform != nil {
		key += "|" + platform.String()
		opts = append(opts, crane.WithPlatform(platform))
	}

	if cached, ok := c.digests[key]; ok {
		return cached.digest, cached.err
	}

	digest, err := crane.Digest(tagName, opts...)
	c.digests[key] = digestResponse{digest: digest, err: err}
	return digest, err
}

func (c *Client) Tags(repoName string) ([]string, error) {
	if cached, ok := c.tags[repoName]; ok {
		return cached.tags, cached.err
	}

	tags, err := crane.ListTags(repoName)
	c.tags[repoName] = tagsResponse{tags: tags, err: err}
	return tags, err
}

func IsNotFound(err error) bool {
	terr, ok := errors.AsType[*transport.Error](err)
	return ok && (terr.StatusCode == http.StatusUnauthorized || terr.StatusCode == http.StatusNotFound)
}
