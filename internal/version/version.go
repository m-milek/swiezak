package version

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
)

type TagVersion struct {
	HasV     bool
	Segments []int
	Suffix   string
}

func ParseVersion(versionString string) (TagVersion, error) {
	afterV, hasV := strings.CutPrefix(versionString, "v")
	semver, suffix, _ := strings.Cut(afterV, "-")

	var segments []int
	for _, segment := range strings.Split(semver, ".") {
		if segment == "" || strings.Trim(segment, "0123456789") != "" {
			return TagVersion{}, fmt.Errorf("not a version: %q", versionString)
		}
		segmentInt, err := strconv.Atoi(segment)
		if err != nil {
			return TagVersion{}, fmt.Errorf("not a version: %q", versionString)
		}
		segments = append(segments, segmentInt)
	}

	return TagVersion{
		HasV:     hasV,
		Segments: segments,
		Suffix:   suffix,
	}, nil
}

func IsCandidateVersionNewer(current, candidate TagVersion) bool {
	for idx := range min(len(current.Segments), len(candidate.Segments)) {
		currentSegment := current.Segments[idx]
		candidateSegment := candidate.Segments[idx]
		if candidateSegment != currentSegment {
			return candidateSegment > currentSegment
		}
	}
	return false
}

func CompareVersions(a, b TagVersion) int {
	for idx := range min(len(a.Segments), len(b.Segments)) {
		if a.Segments[idx] != b.Segments[idx] {
			return cmp.Compare(a.Segments[idx], b.Segments[idx])
		}
	}
	return cmp.Compare(len(b.Segments), len(a.Segments))
}

func FirstDifferingSegment(current, candidate TagVersion) int {
	for idx := range min(len(current.Segments), len(candidate.Segments)) {
		if current.Segments[idx] != candidate.Segments[idx] {
			return idx
		}
	}
	return -1
}
