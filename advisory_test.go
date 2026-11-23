package vers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type npmAdvisoryRange struct {
	TestIndex int    `json:"test_index"`
	Native    string `json:"npm_native"`
}

func TestNPMAdvisoryRangesParse(t *testing.T) {
	filename := filepath.Join("testdata", "local", "data", "npm_advisory.json")
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read %s: %v", filename, err)
	}
	var ranges []npmAdvisoryRange
	if err := json.Unmarshal(data, &ranges); err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	if len(ranges) == 0 {
		t.Fatalf("%s contains no ranges", filename)
	}
	for _, advisoryRange := range ranges {
		t.Run(fmt.Sprintf("%04d", advisoryRange.TestIndex), func(t *testing.T) {
			if _, err := ParseNative(advisoryRange.Native, schemeNPM); err != nil {
				t.Errorf("ParseNative(%q, npm): %v", advisoryRange.Native, err)
			}
		})
	}
}
