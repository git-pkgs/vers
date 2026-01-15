# vers

A Go implementation of the [VERS specification](https://github.com/package-url/purl-spec/blob/main/VERSION-RANGE-SPEC.rst) for version range parsing and comparison across package ecosystems.

## Installation

```bash
go get github.com/git-pkgs/vers
```

## Usage

### Parse VERS URIs

The VERS URI format provides a universal way to express version ranges:

```go
package main

import (
    "fmt"
    "github.com/git-pkgs/vers"
)

func main() {
    // Parse a VERS URI
    r, _ := vers.Parse("vers:npm/>=1.0.0|<2.0.0")

    fmt.Println(r.Contains("1.5.0"))  // true
    fmt.Println(r.Contains("2.0.0"))  // false
    fmt.Println(r.Contains("0.9.0"))  // false
}
```

### Parse Native Package Manager Syntax

Each package ecosystem has its own version constraint syntax. This library parses them all:

```go
// npm: caret, tilde, x-ranges, hyphen ranges
r, _ := vers.ParseNative("^1.2.3", "npm")
r, _ = vers.ParseNative("~1.2.3", "npm")
r, _ = vers.ParseNative("1.0.0 - 2.0.0", "npm")
r, _ = vers.ParseNative(">=1.0.0 <2.0.0", "npm")

// Ruby gems: pessimistic operator
r, _ = vers.ParseNative("~> 1.2", "gem")
r, _ = vers.ParseNative(">= 1.0, < 2.0", "gem")

// Python: compatible release, exclusions
r, _ = vers.ParseNative("~=1.4.2", "pypi")
r, _ = vers.ParseNative(">=1.0.0,<2.0.0,!=1.5.0", "pypi")

// Maven/NuGet: bracket notation
r, _ = vers.ParseNative("[1.0,2.0)", "maven")
r, _ = vers.ParseNative("[1.0,)", "maven")

// Cargo: same syntax as npm
r, _ = vers.ParseNative("^1.2.3", "cargo")

// Go: comma-separated constraints
r, _ = vers.ParseNative(">=1.0.0,<2.0.0", "go")

// Debian: >> and << operators
r, _ = vers.ParseNative(">> 1.0", "deb")

// RPM
r, _ = vers.ParseNative(">= 1.0", "rpm")
```

### Check Version Satisfaction

```go
// Using VERS URI (empty scheme)
ok, _ := vers.Satisfies("1.5.0", "vers:npm/>=1.0.0|<2.0.0", "")
fmt.Println(ok)  // true

// Using native syntax
ok, _ = vers.Satisfies("1.5.0", "^1.0.0", "npm")
fmt.Println(ok)  // true
```

### Compare Versions

```go
vers.Compare("1.2.3", "1.2.4")  // -1 (a < b)
vers.Compare("2.0.0", "1.9.9")  // 1  (a > b)
vers.Compare("1.0.0", "1.0.0")  // 0  (equal)

// Prerelease versions sort before stable
vers.Compare("1.0.0", "1.0.0-alpha")  // 1 (stable > prerelease)
```

### Version Validation and Normalization

```go
vers.Valid("1.2.3")        // true
vers.Valid("invalid")      // false

v, _ := vers.Normalize("1")      // "1.0.0"
v, _ = vers.Normalize("1.2")     // "1.2.0"
v, _ = vers.Normalize("1.2.3")   // "1.2.3"
```

### Create Ranges Programmatically

```go
// Exact version
r := vers.Exact("1.2.3")

// Greater/less than
r = vers.GreaterThan("1.0.0", true)   // >=1.0.0
r = vers.GreaterThan("1.0.0", false)  // >1.0.0
r = vers.LessThan("2.0.0", false)     // <2.0.0

// Unbounded (matches all)
r = vers.Unbounded()

// Combine ranges
r1, _ := vers.ParseNative(">=1.0.0", "npm")
r2, _ := vers.ParseNative("<2.0.0", "npm")
intersection := r1.Intersect(r2)  // >=1.0.0 AND <2.0.0
union := r1.Union(r2)             // >=1.0.0 OR <2.0.0

// Add exclusions
r = r.Exclude("1.5.0")
```

### Convert Back to VERS URI

```go
r, _ := vers.ParseNative("^1.2.3", "npm")
uri := vers.ToVersString(r, "npm")
// vers:npm/>=1.2.3|<2.0.0

// Unbounded range
r = vers.Unbounded()
uri = vers.ToVersString(r, "npm")
// vers:npm/*
```

## Supported Ecosystems

| Ecosystem | Scheme | Example Syntax |
|-----------|--------|----------------|
| npm | `npm` | `^1.2.3`, `~1.2.3`, `>=1.0.0 <2.0.0`, `1.x`, `1.0.0 - 2.0.0` |
| RubyGems | `gem`, `rubygems` | `~> 1.2`, `>= 1.0, < 2.0` |
| PyPI | `pypi` | `~=1.4.2`, `>=1.0.0,<2.0.0`, `!=1.5.0` |
| Maven | `maven` | `[1.0,2.0)`, `(1.0,2.0]`, `[1.0,)`, `[1.0]` |
| NuGet | `nuget` | Same as Maven |
| Cargo | `cargo` | Same as npm |
| Go | `go`, `golang` | `>=1.0.0,<2.0.0` |
| Debian | `deb`, `debian` | `>> 1.0`, `<< 2.0`, `>= 1.0` |
| RPM | `rpm` | `>= 1.0`, `<= 2.0` |

## Development

### Run Tests

```bash
go test ./...
```

### Run Tests with Verbose Output

```bash
go test -v ./...
```

### Run Specific Tests

```bash
go test -v -run TestParseNpmRange
```

### Check Test Coverage

```bash
go test -cover ./...
```

### Generate Coverage Report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Run Benchmarks

```bash
go test -bench=. -benchmem
```

Run specific benchmarks:

```bash
go test -bench=BenchmarkContains -benchmem
```

## License

MIT
