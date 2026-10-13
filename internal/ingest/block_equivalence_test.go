package ingest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func randomValidRow(r *rand.Rand) string {
	f := func() float64 { return r.Float64()*100 - 50 }
	return fmt.Sprintf("146436,%d,G,%g,%g,%g,%g,%g,%g,%g,1,G,%g,%g,%g,%g,%g,%g,%g,-1,%g\n",
		r.IntN(1_000_000_000), f(), f(), f(), f(), f(), f(), f(), f(), f(), f(), f(), f(), f(), f(), f())
}

func randomBadRow(r *rand.Rand) string {
	fields := strings.Split(strings.TrimSuffix(randomValidRow(r), "\n"), ",")
	read := []int{3, 4, 5, 6, 12, 13, 14, 15, 20} // E1 px1 py1 pz1 E2 px2 py2 pz2 M

	switch r.IntN(4) {
	case 0:
		fields = fields[:1+r.IntN(len(fields)-1)]
	case 1:
		fields = append(fields, "extra")
	case 2:
		values := []string{"abc", "NaN", "+Inf", "-Inf", "", "1e999"}
		fields[read[r.IntN(len(read))]] = values[r.IntN(len(values))]
	default:
		fields[read[r.IntN(len(read))]] = `1"2`
	}
	return strings.Join(fields, ",") + "\n"
}

func randomCSV(r *rand.Rand, rows int) (input string, badLines []int) {
	var b strings.Builder
	b.WriteString(header)
	line := 2
	validRows := 0
	for i := 0; i < rows; i++ {
		switch k := r.IntN(15); {
		case k == 0:
			b.WriteString("\n")
		case k <= 2:
			b.WriteString(randomBadRow(r))
			badLines = append(badLines, line)
		default:
			b.WriteString(randomValidRow(r))
			validRows++
		}
		line++
	}

	if validRows == 0 {
		b.WriteString(randomValidRow(r))
		line++
	}
	if len(badLines) == 0 {
		b.WriteString(randomBadRow(r))
		badLines = append(badLines, line)
	}

	input = b.String()
	if r.IntN(2) == 0 {
		input = strings.TrimSuffix(input, "\n")
	}
	return input, badLines
}

func readWithReader(t *testing.T, input string) (events []Event, skipped int) {
	t.Helper()

	rd, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader() returned error: %v", err)
	}
	for {
		ev, err := rd.Read()
		if errors.Is(err, io.EOF) {
			return events, skipped
		}
		if errors.Is(err, ErrMalformedRow) {
			skipped++
			continue
		}
		if err != nil {
			t.Fatalf("Read() returned error: %v", err)
		}
		events = append(events, ev)
	}
}

func countLines(data []byte) int {
	n := bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		n++
	}
	return n
}

func TestBlockPathMatchesReaderOnRandomInput(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))

	for trial := 0; trial < 30; trial++ {
		input, badLines := randomCSV(r, 1+r.IntN(400))
		wantEvents, wantSkipped := readWithReader(t, input)

		if len(wantEvents) == 0 || wantSkipped == 0 || wantSkipped != len(badLines) {
			t.Fatalf("trial %d: reference read %d events and skipped %d rows, generator made %d bad rows",
				trial, len(wantEvents), wantSkipped, len(badLines))
		}

		for _, size := range []int{1, 2, 3, 7, 50, 1000} {
			t.Run(fmt.Sprintf("trial=%d/size=%d", trial, size), func(t *testing.T) {
				br, err := NewBlockReader(strings.NewReader(input), size)
				if err != nil {
					t.Fatalf("NewBlockReader() returned error: %v", err)
				}
				parser := br.Parser()

				var events []Event
				skipped, wantLine := 0, 2
				for {
					b, err := br.ReadBlock()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						t.Fatalf("ReadBlock() returned error: %v", err)
					}

					if b.FirstLine != wantLine {
						t.Fatalf("block starts at line %d, want %d", b.FirstLine, wantLine)
					}
					n := countLines(b.Data)
					wantLine += n

					skip, firstErr := parser.ParseBlock(b, func(ev Event) { events = append(events, ev) })
					skipped += skip

					firstBad := 0
					for _, l := range badLines {
						if l >= b.FirstLine && l < b.FirstLine+n {
							firstBad = l
							break
						}
					}
					switch {
					case firstBad == 0 && firstErr != nil:
						t.Errorf("block at line %d: unexpected error %v", b.FirstLine, firstErr)
					case firstBad != 0 && (firstErr == nil || !strings.Contains(firstErr.Error(), fmt.Sprintf("line %d", firstBad))):
						t.Errorf("block at line %d: error = %v, want it to mention line %d", b.FirstLine, firstErr, firstBad)
					}
				}

				if !slices.Equal(events, wantEvents) {
					t.Errorf("got %d events, want %d (or they differ)", len(events), len(wantEvents))
				}
				if skipped != wantSkipped {
					t.Errorf("skipped %d rows, want %d", skipped, wantSkipped)
				}
			})
		}
	}
}
