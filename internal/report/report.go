package report

import (
	"github.com/m-milek/swiezak/internal/check"
	"github.com/m-milek/swiezak/internal/docker"
)

type Entry struct {
	Image  string       `json:"image"`
	Pinned bool         `json:"pinned"`
	Status check.Status `json:"status"`
	Major  []string     `json:"major,omitempty"`
	Minor  []string     `json:"minor,omitempty"`
	Patch  []string     `json:"patch,omitempty"`
}

func NewEntry(img docker.Image, result check.Result) Entry {
	_, _, pin := img.Reference()
	return Entry{
		Image:  img.ConfigImage,
		Pinned: pin != "",
		Status: result.Status,
		Major:  orEmpty(result.Major),
		Minor:  orEmpty(result.Minor),
		Patch:  orEmpty(result.Patch),
	}
}

func orEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
