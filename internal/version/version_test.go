package version

import (
	"reflect"
	"testing"
)

func mustParse(t *testing.T, s string) TagVersion {
	t.Helper()
	v, err := ParseVersion(s)
	if err != nil {
		t.Fatalf("ParseVersion(%q): %v", s, err)
	}
	return v
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in      string
		want    TagVersion
		wantErr bool
	}{
		{in: "17", want: TagVersion{Segments: []int{17}}},
		{in: "3.20", want: TagVersion{Segments: []int{3, 20}}},
		{in: "1.2.3", want: TagVersion{Segments: []int{1, 2, 3}}},
		{in: "v1.10", want: TagVersion{HasV: true, Segments: []int{1, 10}}},
		{in: "17-alpine", want: TagVersion{Segments: []int{17}, Suffix: "alpine"}},
		{in: "1.2-rc1", want: TagVersion{Segments: []int{1, 2}, Suffix: "rc1"}},
		{in: "17.6-bookworm-slim", want: TagVersion{Segments: []int{17, 6}, Suffix: "bookworm-slim"}},
		{in: "007", want: TagVersion{Segments: []int{7}}},
		{in: "", wantErr: true},
		{in: "v", wantErr: true},
		{in: "latest", wantErr: true},
		{in: "alpine", wantErr: true},
		{in: "+1", wantErr: true},
		{in: "-1", wantErr: true},
		{in: "1..2", wantErr: true},
		{in: "1.", wantErr: true},
		{in: ".1", wantErr: true},
		{in: "1.2a", wantErr: true},
		{in: "V1.2", wantErr: true},
		{in: "1.2+build", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseVersion(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseVersion(%q) = %+v, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVersion(%q): %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseVersion(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsCandidateVersionNewer(t *testing.T) {
	tests := []struct {
		current, candidate string
		want               bool
	}{
		{"17", "18", true},
		{"17", "16", false},
		{"17", "17", false},
		{"17.6", "18", true},
		{"17.6", "17.7", true},
		{"17.6", "17.10", true},
		{"17.6", "17.5", false},
		{"17.6", "17", false},
		{"17", "17.1", false},
		{"17", "18.0", true},
		{"1.2.3", "1.2.4", true},
		{"1.2.3", "1.3", true},
		{"1.2.3", "1.2", false},
	}
	for _, tt := range tests {
		t.Run(tt.current+"->"+tt.candidate, func(t *testing.T) {
			got := IsCandidateVersionNewer(mustParse(t, tt.current), mustParse(t, tt.candidate))
			if got != tt.want {
				t.Errorf("IsCandidateVersionNewer(%q, %q) = %v, want %v", tt.current, tt.candidate, got, tt.want)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"18", "17", 1},
		{"17", "18", -1},
		{"17", "17", 0},
		{"3.10", "3.9", 1},
		{"3.9", "3.10", -1},
		{"18", "18.0", 1},
		{"18.0", "18", -1},
		{"18", "18.1", 1},
		{"18.1", "17.9", 1},
		{"1.2.3", "1.2.3", 0},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := CompareVersions(mustParse(t, tt.a), mustParse(t, tt.b))
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestFirstDifferingSegment(t *testing.T) {
	tests := []struct {
		current, candidate string
		want               int
	}{
		{"17", "18", 0},
		{"17.6", "18", 0},
		{"17.6", "17.7", 1},
		{"1.2.3", "1.2.4", 2},
		{"1.2.3", "1.3", 1},
		{"17", "17", -1},
		{"17", "17.1", -1},
		{"17.6", "17", -1},
	}
	for _, tt := range tests {
		t.Run(tt.current+"->"+tt.candidate, func(t *testing.T) {
			got := FirstDifferingSegment(mustParse(t, tt.current), mustParse(t, tt.candidate))
			if got != tt.want {
				t.Errorf("FirstDifferingSegment(%q, %q) = %d, want %d", tt.current, tt.candidate, got, tt.want)
			}
		})
	}
}

func TestUpgrades(t *testing.T) {
	tests := []struct {
		name    string
		current string
		tags    []string
		want    [3][]string
	}{
		{
			name:    "major only from single segment",
			current: "17",
			tags:    []string{"16", "17", "17.6", "17.6.1", "18", "18.1", "latest", "18-alpine"},
			want:    [3][]string{{"18"}},
		},
		{
			name:    "major and minor",
			current: "17.6",
			tags:    []string{"17.5", "17.6", "17.7", "17.10", "17.6.1", "18", "18.1", "19.0", "17.7-alpine", "v18"},
			want:    [3][]string{{"19.0", "18", "18.1"}, {"17.10", "17.7"}},
		},
		{
			name:    "patch level",
			current: "1.2.3",
			tags:    []string{"1.2.4", "1.2.10", "1.3", "1.3.0", "2", "1.2.3", "1.2.2"},
			want:    [3][]string{{"2"}, {"1.3", "1.3.0"}, {"1.2.10", "1.2.4"}},
		},
		{
			name:    "alpine minors and new major",
			current: "3.20",
			tags:    []string{"3.19", "3.20", "3.20.1", "3.21", "3.24", "3.22.1", "4", "4.0", "4.0.1", "edge", "latest"},
			want:    [3][]string{{"4", "4.0"}, {"3.24", "3.21"}},
		},
		{
			name:    "v prefix must match",
			current: "v1.10",
			tags:    []string{"1.11", "v1.11", "v1.12", "v1.9", "v2"},
			want:    [3][]string{{"v2"}, {"v1.12", "v1.11"}},
		},
		{
			name:    "suffix must match",
			current: "17-alpine",
			tags:    []string{"18", "18-alpine", "19-alpine3.20", "19-bookworm", "18.1-alpine"},
			want:    [3][]string{{"18-alpine"}},
		},
		{
			name:    "date tags rejected",
			current: "17",
			tags:    []string{"20240101", "2024", "100", "18"},
			want:    [3][]string{{"100", "18"}},
		},
		{
			name:    "one extra digit allowed",
			current: "9",
			tags:    []string{"10", "100", "11"},
			want:    [3][]string{{"11", "10"}},
		},
		{
			name:    "fourth segment level ignored",
			current: "1.2.3.4",
			tags:    []string{"1.2.3.5", "1.2.4.0"},
			want:    [3][]string{nil, nil, {"1.2.4.0"}},
		},
		{
			name:    "no tags",
			current: "17",
			tags:    nil,
			want:    [3][]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Upgrades(mustParse(t, tt.current), tt.tags)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Upgrades(%q) = %q, want %q", tt.current, got, tt.want)
			}
		})
	}
}
