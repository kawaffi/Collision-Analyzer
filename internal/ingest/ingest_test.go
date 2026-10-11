package ingest

import (
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"slices"

	"github.com/kawaffi/collision-analyzer/internal/physics"
)

const header = "Run,Event,Type1,E1,px1 ,py1,pz1,pt1,eta1,phi1,Q1,Type2,E2,px2,py2,pz2,pt2,eta2,phi2,Q2,M\n"

const row = "146436,90830792,G,19.1712,3.81713,9.04323,-16.4673,9.81583,-1.28942,1.17139,1,T,5.43984,-0.362592,2.62699,-4.74849,2.65189,-1.34587,1.70796,1,2.73205\n"

func TestReadValidEvent(t *testing.T) {
	r, err := NewReader(strings.NewReader(header + row))
	if err != nil {
		t.Fatalf("NewReader() returned error: %v", err)
	}

	got, err := r.Read()
	if err != nil {
		t.Fatalf("Read() returned error: %v", err)
	}

	want := Event{
		Mu1:             physics.Muon{E: 19.1712, Px: 3.81713, Py: 9.04323, Pz: -16.4673},
		Mu2:             physics.Muon{E: 5.43984, Px: -0.362592, Py: 2.62699, Pz: -4.74849},
		PrecomputedMass: 2.73205,
	}
	if got != want {
		t.Errorf("Read() = %+v, want %+v", got, want)
	}
}

func TestReadReturnsEOFAtEnd(t *testing.T) {
	r, err := NewReader(strings.NewReader(header + row))
	if err != nil {
		t.Fatalf("NewReader() returned error: %v", err)
	}

	if _, err := r.Read(); err != nil {
		t.Fatalf("first Read() returned error: %v", err)
	}
	if _, err := r.Read(); !errors.Is(err, io.EOF) {
		t.Errorf("second Read() error = %v, want io.EOF", err)
	}
}

var firstEvent = Event{
	Mu1:             physics.Muon{E: 19.1712, Px: 3.81713, Py: 9.04323, Pz: -16.4673},
	Mu2:             physics.Muon{E: 5.43984, Px: -0.362592, Py: 2.62699, Pz: -4.74849},
	PrecomputedMass: 2.73205,
}

func TestReadFindsColumnsByName(t *testing.T) {
	const reordered = "M,Type1,E1,px1,py1,pz1,pt1,eta1,phi1,Q1,Type2,E2,px2,py2,pz2,pt2,eta2,phi2,Q2,Run,Event\n" +
		"2.73205,G,19.1712,3.81713,9.04323,-16.4673,9.81583,-1.28942,1.17139,1,T,5.43984,-0.362592,2.62699,-4.74849,2.65189,-1.34587,1.70796,1,146436,90830792\n"

	r, err := NewReader(strings.NewReader(reordered))
	if err != nil {
		t.Fatalf("NewReader() returned error: %v", err)
	}

	got, err := r.Read()
	if err != nil {
		t.Fatalf("Read() returned error: %v", err)
	}
	if got != firstEvent {
		t.Errorf("Read() = %+v, want %+v", got, firstEvent)
	}
}

func TestNewReaderRejectsMissingColumns(t *testing.T) {
	tests := []struct {
		name        string
		header      string
		wantMissing []string
	}{
		{"missing M", "E1,px1,py1,pz1,E2,px2,py2,pz2\n", []string{"M"}},
		{"missing E1 and px2", "py1,px1,pz1,E2,py2,pz2,M\n", []string{"E1", "px2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewReader(strings.NewReader(tt.header))
			if err == nil {
				t.Fatal("NewReader() returned nil error, want an error")
			}
			for _, col := range tt.wantMissing {
				if !strings.Contains(err.Error(), col) {
					t.Errorf("error %q does not mention missing column %q", err, col)
				}
			}
		})
	}
}

func TestNewReaderRejectsEmptyInput(t *testing.T) {
	if _, err := NewReader(strings.NewReader("")); err == nil {
		t.Error("NewReader() accepted empty input, want an error")
	}
}

func TestReadReportsMalformedRow(t *testing.T) {
	tests := []struct {
		name string
		row  string
	}{
		{"non-numeric E1", strings.Replace(row, "19.1712", "abc", 1)},
		{"non-numeric px of second muon", strings.Replace(row, "-0.362592", "x", 1)},
		{"empty mass", strings.Replace(row, "2.73205", "", 1)},
		{"too few fields", "146436,90830792,G\n"},
		{"too many fields", strings.TrimSuffix(row, "\n") + ",extra\n"},
		{"NaN in E1", strings.Replace(row, "19.1712", "NaN", 1)},
		{"NaN in the precomputed mass", strings.Replace(row, "2.73205", "NaN", 1)},
		{"+Inf in pz1", strings.Replace(row, "-16.4673", "+Inf", 1)},
		{"-Inf in E2", strings.Replace(row, "5.43984", "-Inf", 1)},
		{"infinity spelled out", strings.Replace(row, "19.1712", "infinity", 1)},
		{"out of range number", strings.Replace(row, "19.1712", "1e999", 1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewReader(strings.NewReader(header + tt.row))
			if err != nil {
				t.Fatalf("NewReader() returned error: %v", err)
			}

			if _, err := r.Read(); !errors.Is(err, ErrMalformedRow) {
				t.Errorf("Read() error = %v, want ErrMalformedRow", err)
			}
		})
	}
}

func TestReadContinuesAfterMalformedRow(t *testing.T) {
	bad := strings.Replace(row, "19.1712", "abc", 1)
	r, err := NewReader(strings.NewReader(header + bad + row))
	if err != nil {
		t.Fatalf("NewReader() returned error: %v", err)
	}

	if _, err := r.Read(); !errors.Is(err, ErrMalformedRow) {
		t.Fatalf("first Read() error = %v, want ErrMalformedRow", err)
	}
	got, err := r.Read()
	if err != nil {
		t.Fatalf("second Read() returned error: %v", err)
	}
	if got != firstEvent {
		t.Errorf("second Read() = %+v, want %+v", got, firstEvent)
	}
	if _, err := r.Read(); !errors.Is(err, io.EOF) {
		t.Errorf("third Read() error = %v, want io.EOF", err)
	}
}

func TestReadPropagatesIOErrors(t *testing.T) {
	boom := errors.New("disk on fire")
	r, err := NewReader(io.MultiReader(strings.NewReader(header), iotest.ErrReader(boom)))
	if err != nil {
		t.Fatalf("NewReader() returned error: %v", err)
	}

	_, err = r.Read()
	if !errors.Is(err, boom) {
		t.Errorf("Read() error = %v, want %v", err, boom)
	}
	if errors.Is(err, ErrMalformedRow) {
		t.Error("an I/O error was reported as a malformed row")
	}
}

func TestSampleFileMatchesPrecomputedMass(t *testing.T) {
	const (
		wantEvents = 20
		tolerance  = 5e-3
	)

	f, err := os.Open(filepath.Join("..", "..", "testdata", "sample.csv"))
	if err != nil {
		t.Fatalf("opening sample file: %v", err)
	}
	defer f.Close()

	r, err := NewReader(f)
	if err != nil {
		t.Fatalf("NewReader() returned error: %v", err)
	}

	var n int
	var maxDiff float64
	for {
		ev, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("event %d: Read() returned error: %v", n+1, err)
		}
		n++

		got := physics.InvariantMass(ev.Mu1, ev.Mu2)
		diff := math.Abs(got - ev.PrecomputedMass)
		maxDiff = math.Max(maxDiff, diff)
		if diff > tolerance {
			t.Errorf("event %d: InvariantMass() = %v, dataset M = %v (difference %.2e)",
				n, got, ev.PrecomputedMass, diff)
		}
	}

	if n != wantEvents {
		t.Errorf("read %d events, want %d", n, wantEvents)
	}
	t.Logf("largest difference over %d events: %.2e", n, maxDiff)
}

func splitCSV(s string) []string {
	return strings.Split(strings.TrimSuffix(s, "\n"), ",")
}

func newTestParser(t *testing.T) Parser {
	t.Helper()

	p, err := NewParser(splitCSV(header))
	if err != nil {
		t.Fatalf("NewParser() returned error: %v", err)
	}
	return p
}

func TestParseValidRecord(t *testing.T) {
	p := newTestParser(t)

	got, err := p.Parse(Record{Line: 2, Fields: splitCSV(row)})
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if got != firstEvent {
		t.Errorf("Parse() = %+v, want %+v", got, firstEvent)
	}
}

func TestParseRejectsMalformedRecords(t *testing.T) {
	p := newTestParser(t)

	tests := []struct {
		name   string
		fields []string
	}{
		{"non-numeric", splitCSV(strings.Replace(row, "19.1712", "abc", 1))},
		{"NaN", splitCSV(strings.Replace(row, "19.1712", "NaN", 1))},
		{"infinity", splitCSV(strings.Replace(row, "-16.4673", "+Inf", 1))},
		{"empty field", splitCSV(strings.Replace(row, "2.73205", "", 1))},
		{"too few fields", splitCSV(row)[:3]},
		{"too many fields", append(splitCSV(row), "extra")},
		{"no fields", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.Parse(Record{Line: 7, Fields: tt.fields})

			if !errors.Is(err, ErrMalformedRow) {
				t.Fatalf("Parse() error = %v, want ErrMalformedRow", err)
			}
			if !strings.Contains(err.Error(), "line 7") {
				t.Errorf("error %q does not mention the line number", err)
			}
		})
	}
}

func TestReadRecordReturnsRawFieldsAndLineNumbers(t *testing.T) {
	r, err := NewReader(strings.NewReader(header + row + row))
	if err != nil {
		t.Fatalf("NewReader() returned error: %v", err)
	}

	for wantLine := 2; wantLine <= 3; wantLine++ {
		rec, err := r.ReadRecord()
		if err != nil {
			t.Fatalf("ReadRecord() for line %d returned error: %v", wantLine, err)
		}
		if rec.Line != wantLine {
			t.Errorf("Line = %d, want %d", rec.Line, wantLine)
		}
		if want := splitCSV(row); !slices.Equal(rec.Fields, want) {
			t.Errorf("Fields = %v, want %v", rec.Fields, want)
		}
	}

	if _, err := r.ReadRecord(); !errors.Is(err, io.EOF) {
		t.Errorf("ReadRecord() at the end: error = %v, want io.EOF", err)
	}
}

func TestReadRecordReportsBadFieldCountAndContinues(t *testing.T) {
	r, err := NewReader(strings.NewReader(header + "146436,90830792,G\n" + row))
	if err != nil {
		t.Fatalf("NewReader() returned error: %v", err)
	}

	if _, err := r.ReadRecord(); !errors.Is(err, ErrMalformedRow) {
		t.Fatalf("first ReadRecord() error = %v, want ErrMalformedRow", err)
	}
	rec, err := r.ReadRecord()
	if err != nil {
		t.Fatalf("second ReadRecord() returned error: %v", err)
	}
	if rec.Line != 3 {
		t.Errorf("Line = %d, want 3", rec.Line)
	}
}

func TestReaderParserParsesItsOwnRecords(t *testing.T) {
	r, err := NewReader(strings.NewReader(header + row))
	if err != nil {
		t.Fatalf("NewReader() returned error: %v", err)
	}

	rec, err := r.ReadRecord()
	if err != nil {
		t.Fatalf("ReadRecord() returned error: %v", err)
	}
	got, err := r.Parser().Parse(rec)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if got != firstEvent {
		t.Errorf("Parse() = %+v, want %+v", got, firstEvent)
	}
}

func TestReadRecordDoesNotReuseMemory(t *testing.T) {
	second := strings.Replace(row, "19.1712", "20.1712", 1)
	r, err := NewReader(strings.NewReader(header + row + second))
	if err != nil {
		t.Fatalf("NewReader() returned error: %v", err)
	}

	first, err := r.ReadRecord()
	if err != nil {
		t.Fatalf("first ReadRecord() returned error: %v", err)
	}
	if _, err := r.ReadRecord(); err != nil {
		t.Fatalf("second ReadRecord() returned error: %v", err)
	}

	if got := first.Fields[3]; got != "19.1712" {
		t.Errorf("first record changed after reading the next one: E1 = %q, want %q", got, "19.1712")
	}
}
