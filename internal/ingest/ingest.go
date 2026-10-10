package ingest

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/kawaffi/collision-analyzer/internal/physics"
)

var ErrMalformedRow = errors.New("malformed row")

type Event struct {
	Mu1, Mu2 physics.Muon
	// PrecomputedMass is the invariant mass stored in the dataset. It is meant
	// as a test oracle, not as the value the pipeline reports.
	PrecomputedMass float64
}

type Reader struct {
	csv  *csv.Reader
	cols columns
}

type columns struct {
	e1, px1, py1, pz1 int
	e2, px2, py2, pz2 int
	mass              int
}

func NewReader(r io.Reader) (*Reader, error) {
	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("ingest: reading header: %w", err)
	}

	cols, err := locateColumns(header)
	if err != nil {
		return nil, err
	}

	return &Reader{csv: cr, cols: cols}, nil
}

func (r *Reader) Read() (Event, error) {
	rec, err := r.csv.Read()
	if err != nil {
		var parseErr *csv.ParseError
		if errors.As(err, &parseErr) {
			return Event{}, fmt.Errorf("%w: %w", ErrMalformedRow, err)
		}
		return Event{}, err
	}

	line, _ := r.csv.FieldPos(0)
	var rowErr error
	f := func(name string, i int) float64 {
		v, err := strconv.ParseFloat(rec[i], 64)
		if (err != nil || !isFinite(v)) && rowErr == nil {
			rowErr = fmt.Errorf("%w: line %d, column %s: %q is not a finite number",
				ErrMalformedRow, line, name, rec[i])
		}
		return v
	}

	c := r.cols
	ev := Event{
		Mu1:             physics.Muon{E: f("E1", c.e1), Px: f("px1", c.px1), Py: f("py1", c.py1), Pz: f("pz1", c.pz1)},
		Mu2:             physics.Muon{E: f("E2", c.e2), Px: f("px2", c.px2), Py: f("py2", c.py2), Pz: f("pz2", c.pz2)},
		PrecomputedMass: f("M", c.mass),
	}
	if rowErr != nil {
		return Event{}, rowErr
	}
	return ev, nil
}

func locateColumns(header []string) (columns, error) {
	index := make(map[string]int, len(header))
	for i, name := range header {
		index[strings.TrimSpace(name)] = i
	}

	var missing []string
	find := func(name string) int {
		i, ok := index[name]
		if !ok {
			missing = append(missing, name)
		}
		return i
	}

	cols := columns{
		e1: find("E1"), px1: find("px1"), py1: find("py1"), pz1: find("pz1"),
		e2: find("E2"), px2: find("px2"), py2: find("py2"), pz2: find("pz2"),
		mass: find("M"),
	}
	if len(missing) > 0 {
		return columns{}, fmt.Errorf("ingest: missing required columns: %s", strings.Join(missing, ", "))
	}
	return cols, nil
}

func isFinite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}
