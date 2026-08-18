package vers

import (
	"slices"
	"testing"
)

func TestTagCandidates(t *testing.T) {
	tests := []struct {
		name    string
		version string
		scheme  string
		want    []string
	}{
		{
			name:    "normalized npm version",
			version: "1.2",
			scheme:  "npm",
			want:    []string{"1.2", "v1.2", "1.2.0", "v1.2.0"},
		},
		{
			name:    "already normalized",
			version: "1.2.3",
			scheme:  "npm",
			want:    []string{"1.2.3", "v1.2.3"},
		},
		{
			name:    "go v prefix",
			version: "v1.2",
			scheme:  "golang",
			want:    []string{"v1.2", "1.2", "v1.2.0", "1.2.0"},
		},
		{
			name:    "uppercase v prefix",
			version: "V1.2",
			scheme:  "golang",
			want:    []string{"V1.2", "1.2", "v1.2.0", "1.2.0"},
		},
		{
			name:    "numeric gem version",
			version: "4.0",
			scheme:  "gem",
			want:    []string{"4.0", "v4.0"},
		},
		{
			name:    "decorated gem version",
			version: "v4.0",
			scheme:  "gem",
			want:    []string{"v4.0", "4.0"},
		},
		{
			name:    "composer branch version",
			version: "dev-main",
			scheme:  "composer",
			want:    []string{"dev-main"},
		},
		{
			name:    "surrounding whitespace",
			version: " 1.2.3 ",
			scheme:  "npm",
			want:    []string{"1.2.3", "v1.2.3"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := TagCandidates(test.version, test.scheme)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("TagCandidates(%q, %q) = %q, want %q",
					test.version, test.scheme, got, test.want)
			}
		})
	}
}

func TestTagCandidatesRejectsInvalidVersion(t *testing.T) {
	if candidates, err := TagCandidates("not a version", "npm"); err == nil || candidates != nil {
		t.Fatalf("TagCandidates() = (%q, %v), want (nil, error)", candidates, err)
	}
}
