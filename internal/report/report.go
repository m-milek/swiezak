package report

import (
	"context"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/m-milek/swiezak/internal/check"
	"github.com/m-milek/swiezak/internal/docker"
	"github.com/m-milek/swiezak/internal/registry"
)

type Entry struct {
	Image     string       `json:"image"`
	Pinned    bool         `json:"pinned"`
	Status    check.Status `json:"status"`
	Major     []string     `json:"major,omitempty"`
	Minor     []string     `json:"minor,omitempty"`
	Patch     []string     `json:"patch,omitempty"`
	Error     string       `json:"error,omitempty"`
	CheckedAt time.Time    `json:"checkedAt"`
}

func NewEntry(img docker.Image, result check.Result, checkedAt time.Time) Entry {
	_, _, pin := img.Reference()
	return Entry{
		Image:     img.ConfigImage,
		Pinned:    pin != "",
		Status:    result.Status,
		Major:     orEmpty(result.Major),
		Minor:     orEmpty(result.Minor),
		Patch:     orEmpty(result.Patch),
		Error:     result.Error,
		CheckedAt: checkedAt,
	}
}

func orEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func Build(ctx context.Context, keychain authn.Keychain) ([]Entry, error) {
	images, err := docker.ListImages(ctx)
	if err != nil {
		return nil, err
	}

	registryClient := registry.NewClient(keychain)
	entries := []Entry{}
	for _, img := range images {
		entries = append(entries, NewEntry(img, check.Image(img, registryClient), time.Now().UTC()))
	}
	return entries, nil
}
