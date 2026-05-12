package vers

import "testing"

func TestHighestSatisfying_Npm(t *testing.T) {
	versions := []string{"1.0.0", "1.5.0", "2.0.0", "2.5.0", "3.0.0"}
	cases := []struct {
		constraint string
		want       string
	}{
		{"^1.0", "1.5.0"},
		{"^2.0.0", "2.5.0"},
		{"~2.0.0", "2.0.0"},
		{">=1.0.0 <2.0.0", "1.5.0"},
		{"^4.0.0", ""}, // no satisfying version
	}
	for _, tc := range cases {
		got, err := HighestSatisfying(versions, tc.constraint, "npm")
		if err != nil {
			t.Errorf("HighestSatisfying(%q): %v", tc.constraint, err)
			continue
		}
		if got != tc.want {
			t.Errorf("HighestSatisfying(%q) = %q, want %q", tc.constraint, got, tc.want)
		}
	}
}

func TestHighestSatisfying_OrderIndependent(t *testing.T) {
	// Same set, different order — picks the same highest.
	a := []string{"1.0.0", "2.0.0", "1.5.0"}
	b := []string{"2.0.0", "1.5.0", "1.0.0"}
	gotA, _ := HighestSatisfying(a, "^1.0", "npm")
	gotB, _ := HighestSatisfying(b, "^1.0", "npm")
	if gotA != "1.5.0" || gotB != "1.5.0" {
		t.Errorf("order mismatch: a=%q b=%q", gotA, gotB)
	}
}

func TestHighestSatisfying_SkipsInvalidVersions(t *testing.T) {
	// Garbage versions should be skipped, not stop the walk.
	versions := []string{"not-a-version", "1.0.0", "also-bad", "1.5.0"}
	got, err := HighestSatisfying(versions, "^1.0", "npm")
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.5.0" {
		t.Errorf("got %q, want 1.5.0", got)
	}
}

func TestHighestSatisfying_VersURI(t *testing.T) {
	// Empty scheme → constraint is a vers URI rather than native syntax.
	got, err := HighestSatisfying(
		[]string{"1.0.0", "1.5.0", "2.0.0"},
		"vers:npm/>=1.0.0|<2.0.0",
		"")
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.5.0" {
		t.Errorf("got %q, want 1.5.0", got)
	}
}

func TestHighestSatisfying_Empty(t *testing.T) {
	got, err := HighestSatisfying(nil, "^1.0", "npm")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
