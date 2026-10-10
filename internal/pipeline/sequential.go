package pipeline

import (
	"errors"
	"io"

	"github.com/kawaffi/collision-analyzer/internal/histogram"
	"github.com/kawaffi/collision-analyzer/internal/ingest"
	"github.com/kawaffi/collision-analyzer/internal/physics"
)

type Stats struct {
	Processed uint64
	Skipped   uint64
}

func Sequential(r io.Reader, h *histogram.Histogram) (Stats, error) {
	rd, err := ingest.NewReader(r)
	if err != nil {
		return Stats{}, err
	}

	var stats Stats
	for {
		ev, err := rd.Read()
		if errors.Is(err, io.EOF) {
			return stats, nil
		}
		if errors.Is(err, ingest.ErrMalformedRow) {
			stats.Skipped++
			continue
		}
		if err != nil {
			return stats, err
		}

		h.Add(physics.InvariantMass(ev.Mu1, ev.Mu2))
		stats.Processed++
	}
}
