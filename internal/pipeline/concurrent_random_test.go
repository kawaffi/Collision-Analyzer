package pipeline

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/kawaffi/collision-analyzer/internal/histogram"
)

func randomRow(r *rand.Rand) string {
	const muonMass = 0.1057
	muon := func() (e, px, py, pz float64) {
		px, py, pz = r.Float64()*100-50, r.Float64()*100-50, r.Float64()*100-50
		e = math.Sqrt(px*px + py*py + pz*pz + muonMass*muonMass)
		return
	}
	e1, px1, py1, pz1 := muon()
	e2, px2, py2, pz2 := muon()

	return fmt.Sprintf("146436,%d,G,%g,%g,%g,%g,0,0,0,1,G,%g,%g,%g,%g,0,0,0,-1,0\n",
		r.IntN(1_000_000_000), e1, px1, py1, pz1, e2, px2, py2, pz2)
}

func randomBadRow(r *rand.Rand) string {
	fields := strings.Split(strings.TrimSuffix(randomRow(r), "\n"), ",")

	switch r.IntN(3) {
	case 0:
		fields = fields[:1+r.IntN(len(fields)-1)]
	case 1:
		fields = append(fields, "extra")
	default:
		columns := []int{3, 4, 5, 6, 12, 13, 14, 15, 20} // E1 px1 py1 pz1 E2 px2 py2 pz2 M
		values := []string{"abc", "NaN", "+Inf", "-Inf", "", "1e999"}
		fields[columns[r.IntN(len(columns))]] = values[r.IntN(len(values))]
	}
	return strings.Join(fields, ",") + "\n"
}

func randomInput(r *rand.Rand, rows int) string {
	var b strings.Builder
	b.WriteString(header)
	for i := 0; i < rows; i++ {
		if r.IntN(10) == 0 {
			b.WriteString(randomBadRow(r))
		} else {
			b.WriteString(randomRow(r))
		}
	}
	return b.String()
}

func TestConcurrentMatchesSequentialOnRandomInput(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))

	newHist := func() *histogram.Histogram {
		h, err := histogram.New(0, 120, 60)
		if err != nil {
			t.Fatalf("histogram.New() returned error: %v", err)
		}
		return h
	}

	for trial := 0; trial < 10; trial++ {
		input := randomInput(r, 1+r.IntN(3000))

		want := newHist()
		wantStats, err := Sequential(strings.NewReader(input), want)
		if err != nil {
			t.Fatalf("trial %d: Sequential() returned error: %v", trial, err)
		}

		used := 0
		for _, c := range want.Counts() {
			if c > 0 {
				used++
			}
		}
		if wantStats.Processed == 0 || wantStats.Skipped == 0 || used < 5 || want.Overflow() == 0 {
			t.Fatalf("trial %d: input too poor: stats %+v, %d bins used, overflow %d",
				trial, wantStats, used, want.Overflow())
		}

		for _, workers := range []int{1, 2, 3, 8} {
			for _, batchSize := range []int{1, 17, 500, 5000} {
				t.Run(fmt.Sprintf("trial=%d/workers=%d/batch=%d", trial, workers, batchSize), func(t *testing.T) {
					got := newHist()

					stats, err := Concurrent(context.Background(), strings.NewReader(input), got,
						Options{Workers: workers, BatchSize: batchSize})
					if err != nil {
						t.Fatalf("Concurrent() returned error: %v", err)
					}

					if stats != wantStats {
						t.Errorf("Concurrent() stats = %+v, want %+v", stats, wantStats)
					}
					assertSameHistogram(t, got, want)
				})
			}
		}
	}
}
