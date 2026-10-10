package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/kawaffi/collision-analyzer/internal/physics"
)

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

	index := make(map[string]int, len(header))
	for i, name := range header {
		index[strings.TrimSpace(name)] = i
	}

	return &Reader{
		csv: cr,
		cols: columns{
			e1: index["E1"], px1: index["px1"], py1: index["py1"], pz1: index["pz1"],
			e2: index["E2"], px2: index["px2"], py2: index["py2"], pz2: index["pz2"],
			mass: index["M"],
		},
	}, nil
}

func (r *Reader) Read() (Event, error) {
	rec, err := r.csv.Read()
	if err != nil {
		return Event{}, err
	}

	f := func(i int) float64 {
		v, _ := strconv.ParseFloat(rec[i], 64)
		return v
	}

	c := r.cols
	return Event{
		Mu1:             physics.Muon{E: f(c.e1), Px: f(c.px1), Py: f(c.py1), Pz: f(c.pz1)},
		Mu2:             physics.Muon{E: f(c.e2), Px: f(c.px2), Py: f(c.py2), Pz: f(c.pz2)},
		PrecomputedMass: f(c.mass),
	}, nil
}
