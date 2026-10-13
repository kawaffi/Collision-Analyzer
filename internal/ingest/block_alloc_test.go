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

func TestParseBlockReusesTheFieldSlice(t *testing.T) {
	p := newTestParser(t)
	data := []byte(strings.Repeat(row, allocRows))

	base := csvAllocs(data)
	got := parseBlockAllocs(p, Block{FirstLine: 2, Data: data})

	// Reusing the slice of fields saves one allocation per row.
	if want := base - allocRows + allocSlack; got > want {
		t.Errorf("ParseBlock makes %.0f allocations for %d rows, want at most %.0f: it should save one per row against encoding/csv (%.0f)",
			got, allocRows, want, base)
	}
}
