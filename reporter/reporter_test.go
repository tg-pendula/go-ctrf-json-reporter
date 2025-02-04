package reporter

import (
	"fmt"
	"testing"
)

func TestGetCoverage(t *testing.T) {
	expectedCoverage := float32(94.1)
	event := TestEvent{
		Time:    "2025-02-04T11:02:26.181998991+10:00",
		Action:  "output",
		Package: "github.com/ctrf-io/go-ctrf-json-reporter/ctrf",
		Test:    "",
		Elapsed: 0,
		Output:  fmt.Sprintf("coverage: %.2f%% of statements\n", expectedCoverage),
	}

	c, err := getCoverage(event)

	if err != nil {
		t.Error(err)
	}

	if c == nil {
		t.Errorf("expected non-nil coverageInfo")
		return
	}

	if c.Coverage != expectedCoverage {
		t.Errorf("want: %.2f got: %.2f", expectedCoverage, c.Coverage)
	}

	if c.Suite != event.Package {
		t.Errorf("want: %s, got: %s", event.Package, c.Suite)
	}
}
