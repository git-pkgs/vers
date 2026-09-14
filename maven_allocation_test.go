package vers

import (
	"fmt"
	"testing"
)

func TestMavenComponentNormalization(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want int
	}{
		{"1.0.0-rc1", "1-rc1", 0},
		{"1.0.0-alpha.0.1", "1-alpha.0.1", 0},
		{"1.0.0-final", "1", 0},
		{"0.0.0-rc1", "0-rc1", 0},
		{"1.0.0.2-rc1", "1.0.0.2", -1},
		{"1.0.0-sp1", "1", 1},
		{"1.0.0-RC1", "1.0.0-rc1", 0},
		{"1.0.0-a1", "1-alpha1", 0},
		{"1.0.0", "1", 0},
		{"0.0.0", "0", 0},
		{"1.9999999999999999999999-rc1", "1.2-rc1", 1},
	} {
		t.Run(tt.a+"/"+tt.b, func(t *testing.T) {
			if got := CompareWithScheme(tt.a, tt.b, "maven"); got != tt.want {
				t.Fatalf("compare = %d, want %d", got, tt.want)
			}
			if got := CompareWithScheme(tt.b, tt.a, "maven"); got != -tt.want {
				t.Fatalf("reverse compare = %d, want %d", got, -tt.want)
			}
		})
	}
	versions := []string{"1.0.0-alpha1", "1.0.0-rc1", "1.0.0", "1.0.0-sp1", "2.0.0"}
	got, err := HighestSatisfying(versions, "[1.0,2.0)", "maven")
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.0.0-sp1" {
		t.Fatalf("highest = %q", got)
	}
	for _, version := range []string{"1.0.0-alpha1", "1.0.0-rc1"} {
		if !IsPrereleaseWithScheme(version, "maven") {
			t.Fatalf("%q should be a prerelease", version)
		}
	}
}

func BenchmarkMavenComponents(b *testing.B) {
	for _, tt := range []struct{ name, a, other string }{
		{"release", "1.2.3", "1.2.4"},
		{"prerelease", "1.2.3-rc1", "1.2.3"},
		{"trailing_zeros", "1.0.0-rc1", "1.0.0"},
	} {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				CompareWithScheme(tt.a, tt.other, "maven")
			}
		})
	}
	b.Run("highest_500", func(b *testing.B) {
		versions := make([]string, 500)
		for i := range versions {
			versions[i] = fmt.Sprintf("1.%d.0-rc1", i)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			got, err := HighestSatisfying(versions, "[1.0,2.0)", "maven")
			if err != nil {
				b.Fatal(err)
			}
			if got != "1.499.0-rc1" {
				b.Fatalf("highest = %q", got)
			}
		}
	})
}
