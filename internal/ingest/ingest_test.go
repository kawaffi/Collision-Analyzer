package ingest

import (
	"errors"
	"io"
	"strings"
	"testing"

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
