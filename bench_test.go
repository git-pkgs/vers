package vers

import "testing"

// Parsing benchmarks

func BenchmarkParse_VersURI_Simple(b *testing.B) {
	for b.Loop() {
		_, _ = Parse("vers:npm/>=1.2.3")
	}
}

func BenchmarkParse_VersURI_Complex(b *testing.B) {
	for b.Loop() {
		_, _ = Parse("vers:npm/>=1.2.3|<2.0.0|!=1.5.0")
	}
}

func BenchmarkParseNative_Npm_Caret(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative("^1.2.3", "npm")
	}
}

func BenchmarkParseNative_Npm_Tilde(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative("~1.2.3", "npm")
	}
}

func BenchmarkParseNative_Npm_Range(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative(">=1.0.0 <2.0.0", "npm")
	}
}

func BenchmarkParseNative_Npm_Or(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative("^1.0.0 || ^2.0.0 || ^3.0.0", "npm")
	}
}

func BenchmarkParseNative_Gem_Pessimistic(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative("~> 1.2.3", "gem")
	}
}

func BenchmarkParseNative_Pypi_Compatible(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative("~=1.4.2", "pypi")
	}
}

func BenchmarkParseNative_Maven_Bracket(b *testing.B) {
	for b.Loop() {
		_, _ = ParseNative("[1.0,2.0)", "maven")
	}
}

// Contains benchmarks

func BenchmarkContains_Simple(b *testing.B) {
	r, _ := ParseNative("^1.2.3", "npm")
	b.ResetTimer()
	for b.Loop() {
		r.Contains("1.5.0")
	}
}

func BenchmarkContains_MultiInterval(b *testing.B) {
	r, _ := ParseNative("^1.0.0 || ^2.0.0 || ^3.0.0", "npm")
	b.ResetTimer()
	for b.Loop() {
		r.Contains("2.5.0")
	}
}

func BenchmarkContains_WithExclusions(b *testing.B) {
	r, _ := Parse("vers:npm/>=1.0.0|<2.0.0|!=1.5.0|!=1.6.0|!=1.7.0")
	b.ResetTimer()
	for b.Loop() {
		r.Contains("1.8.0")
	}
}

func BenchmarkContains_Prerelease(b *testing.B) {
	r, _ := ParseNative(">=1.0.0-alpha.1", "npm")
	b.ResetTimer()
	for b.Loop() {
		r.Contains("1.0.0-beta.2")
	}
}

func BenchmarkCompare_Simple(b *testing.B) {
	for b.Loop() {
		Compare("1.2.3", "1.2.4")
	}
}

func BenchmarkCompare_Prerelease(b *testing.B) {
	for b.Loop() {
		Compare("1.0.0-alpha.1", "1.0.0-beta.2")
	}
}

func BenchmarkCompareWithScheme(b *testing.B) {
	tests := []struct {
		name, a, other, scheme string
	}{
		{name: "Semver", a: "1.2.3-alpha.1", other: "1.2.3-beta.2", scheme: "npm"},
		{name: "Gem", a: "1.2.3.pre.1", other: "1.2.3", scheme: "gem"},
		{name: "PyPI", a: "1.2.3rc1", other: "1.2.3", scheme: "pypi"},
		{name: "Maven", a: "1.2.3-rc1", other: "1.2.3", scheme: "maven"},
		{name: "NuGet", a: "1.2.3-alpha.1", other: "1.2.3", scheme: "nuget"},
		{name: "Debian", a: "1:1.2.3-1", other: "1:1.2.3-2", scheme: "deb"},
		{name: "RPM", a: "1:1.2.3-1", other: "1:1.2.3-2", scheme: "rpm"},
		// The remaining schemes have their own comparators rather than
		// sharing the SemVer path, so allocation regressions in them only
		// show up if they are measured here.
		{name: "ALPM", a: "1.2.3-1", other: "1.2.4-1", scheme: "alpm"},
		{name: "ALPMEpoch", a: "1:1.2.3-1", other: "2:1.2.3-1", scheme: "alpm"},
		{name: "Conan", a: "1.2.3-rc1", other: "1.2.3", scheme: "conan"},
		{name: "ConanBuild", a: "1.2.0-alpha+build1", other: "1.2-alpha+build2", scheme: "conan"},
		{name: "Gentoo", a: "1.2.3_rc1", other: "1.2.3", scheme: "gentoo"},
		{name: "GentooRevision", a: "1.2.3a_p1-r2", other: "1.2.3a_p2-r1", scheme: "gentoo"},
		{name: "APK", a: "1.2.3_rc1", other: "1.2.3", scheme: "apk"},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for b.Loop() {
				CompareWithScheme(tt.a, tt.other, tt.scheme)
			}
		})
	}
}

func BenchmarkParseConstraint(b *testing.B) {
	for b.Loop() {
		_, _ = ParseConstraint(">=1.2.3")
	}
}

func BenchmarkParseVersion_Cached(b *testing.B) {
	_, _ = ParseVersion("1.2.3-alpha.1+build.5")
	b.ResetTimer()
	for b.Loop() {
		_, _ = ParseVersion("1.2.3-alpha.1+build.5")
	}
}

func BenchmarkValid_Simple(b *testing.B) {
	_, _ = ParseVersion("1.2.3")
	b.ResetTimer()
	for b.Loop() {
		Valid("1.2.3")
	}
}

// Range operation benchmarks

func BenchmarkUnion_TwoRanges(b *testing.B) {
	r1, _ := ParseNative("^1.0.0", "npm")
	r2, _ := ParseNative("^2.0.0", "npm")
	b.ResetTimer()
	for b.Loop() {
		r1.Union(r2)
	}
}

func BenchmarkUnion_ManyRanges(b *testing.B) {
	ranges := make([]*Range, 10)
	for i := range ranges {
		ranges[i], _ = ParseNative("^1.0.0", "npm")
	}
	b.ResetTimer()
	for b.Loop() {
		result := ranges[0]
		for _, r := range ranges[1:] {
			result = result.Union(r)
		}
	}
}

func BenchmarkIntersect_TwoRanges(b *testing.B) {
	r1, _ := ParseNative(">=1.0.0", "npm")
	r2, _ := ParseNative("<2.0.0", "npm")
	b.ResetTimer()
	for b.Loop() {
		r1.Intersect(r2)
	}
}

func BenchmarkIntersect_ManyRanges(b *testing.B) {
	r1, _ := ParseNative(">=1.0.0", "npm")
	r2, _ := ParseNative("<3.0.0", "npm")
	r3, _ := ParseNative(">=1.5.0", "npm")
	r4, _ := ParseNative("<2.5.0", "npm")
	b.ResetTimer()
	for b.Loop() {
		r1.Intersect(r2).Intersect(r3).Intersect(r4)
	}
}

// Satisfies benchmarks (combines parsing and contains)

func BenchmarkSatisfies_VersURI(b *testing.B) {
	for b.Loop() {
		_, _ = Satisfies("1.5.0", "vers:npm/>=1.0.0|<2.0.0", "")
	}
}

func BenchmarkSatisfies_Native(b *testing.B) {
	for b.Loop() {
		_, _ = Satisfies("1.5.0", "^1.2.3", "npm")
	}
}

func BenchmarkHighestSatisfying_Npm(b *testing.B) {
	versions := []string{
		"1.0.0", "1.1.0", "1.2.0", "1.3.0", "1.4.0", "1.5.0", "1.6.0", "1.7.0",
		"1.8.0", "1.9.0", "2.0.0-alpha.1", "2.0.0", "2.1.0", "3.0.0",
	}
	for b.Loop() {
		_, _ = HighestSatisfying(versions, "^1.2.0", "npm")
	}
}
