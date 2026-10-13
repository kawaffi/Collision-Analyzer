package ingest

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

const (
	allocRows  = 1000
	allocSlack = 20
)

func csvAllocs(data []byte) float64 {
	return testing.AllocsPerRun(10, func() {
		cr := csv.NewReader(bytes.NewReader(data))
		cr.FieldsPerRecord = -1
		for {
			if _, err := cr.Read(); err != nil {
				return
			}
		}
	})
}

func parseBlockAllocs(p Parser, b Block) float64 {
	return testing.AllocsPerRun(10, func() {
		p.ParseBlock(b, func(Event) {})
	})
}

func TestParseBlockDoesNotAllocateBeyondCSV(t *testing.T) {
	p := newTestParser(t)
	data := []byte(strings.Repeat(row, allocRows))

	base := csvAllocs(data)
	got := parseBlockAllocs(p, Block{FirstLine: 2, Data: data})

	if got > base+allocSlack {
		t.Errorf("ParseBlock makes %.0f allocations for %d rows, encoding/csv alone makes %.0f (%.2f extra per row)",
			got, allocRows, base, (got-base)/allocRows)
	}
}
