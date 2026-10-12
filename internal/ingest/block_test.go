package ingest

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func readAllBlocks(t *testing.T, input string, size int) []Block {
	t.Helper()

	br, err := NewBlockReader(strings.NewReader(input), size)
	if err != nil {
		t.Fatalf("NewBlockReader() returned error: %v", err)
	}

	var blocks []Block
	for {
		b, err := br.ReadBlock()
		if errors.Is(err, io.EOF) {
			return blocks
		}
		if err != nil {
			t.Fatalf("ReadBlock() returned error: %v", err)
		}
		blocks = append(blocks, b)
	}
}

func TestBlockReaderSplitsRowsIntoBlocks(t *testing.T) {
	tests := []struct {
		name           string
		rows, size     int
		wantFirstLines []int
		wantRows       []int // rows in each block
	}{
		{"one row per block", 3, 1, []int{2, 3, 4}, []int{1, 1, 1}},
		{"last block is partial", 5, 2, []int{2, 4, 6}, []int{2, 2, 1}},
		{"exact multiple", 4, 2, []int{2, 4}, []int{2, 2}},
		{"block larger than input", 3, 100, []int{2}, []int{3}},
		{"no rows", 0, 5, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := readAllBlocks(t, header+strings.Repeat(row, tt.rows), tt.size)

			if len(blocks) != len(tt.wantFirstLines) {
				t.Fatalf("got %d blocks, want %d", len(blocks), len(tt.wantFirstLines))
			}
			for i, b := range blocks {
				if b.FirstLine != tt.wantFirstLines[i] {
					t.Errorf("block %d: FirstLine = %d, want %d", i, b.FirstLine, tt.wantFirstLines[i])
				}
				if want := strings.Repeat(row, tt.wantRows[i]); string(b.Data) != want {
					t.Errorf("block %d: Data = %q, want %d rows", i, b.Data, tt.wantRows[i])
				}
			}
		})
	}
}

func TestBlockReaderHandlesMissingFinalNewline(t *testing.T) {
	last := strings.TrimSuffix(row, "\n")

	for _, size := range []int{1, 2, 10} {
		var got strings.Builder
		for _, b := range readAllBlocks(t, header+row+last, size) {
			got.Write(b.Data)
		}
		if got.String() != row+last {
			t.Errorf("size %d: data = %q, want %q", size, got.String(), row+last)
		}
	}

	if blocks := readAllBlocks(t, strings.TrimSuffix(header, "\n"), 5); len(blocks) != 0 {
		t.Errorf("header only: got %d blocks, want 0", len(blocks))
	}
}

func TestBlockReaderCountsBlankLines(t *testing.T) {
	blocks := readAllBlocks(t, header+row+"\n"+row, 2)

	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
	if blocks[0].FirstLine != 2 || string(blocks[0].Data) != row+"\n" {
		t.Errorf("block 0 = %+v", blocks[0])
	}
	if blocks[1].FirstLine != 4 || string(blocks[1].Data) != row {
		t.Errorf("block 1 = %+v", blocks[1])
	}
}

func TestBlockReaderHandlesVeryLongLines(t *testing.T) {
	long := strings.Replace(row, ",G,", ","+strings.Repeat("x", 300_000)+",", 1)

	blocks := readAllBlocks(t, header+long+row, 1)

	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
	if string(blocks[0].Data) != long {
		t.Errorf("the long line was cut: got %d bytes, want %d", len(blocks[0].Data), len(long))
	}
	if string(blocks[1].Data) != row {
		t.Errorf("the line after the long one is wrong: %q", blocks[1].Data)
	}
}

func TestBlockReaderBlocksDoNotShareMemory(t *testing.T) {
	second := strings.Replace(row, "19.1712", "20.1712", 1)
	br, err := NewBlockReader(strings.NewReader(header+row+second), 1)
	if err != nil {
		t.Fatalf("NewBlockReader() returned error: %v", err)
	}

	first, err := br.ReadBlock()
	if err != nil {
		t.Fatalf("first ReadBlock() returned error: %v", err)
	}
	if _, err := br.ReadBlock(); err != nil {
		t.Fatalf("second ReadBlock() returned error: %v", err)
	}

	if string(first.Data) != row {
		t.Errorf("first block changed after reading the next one: %q", first.Data)
	}
}

func TestNewBlockReaderRejectsBadInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		size  int
	}{
		{"empty input", "", 10},
		{"missing columns", "E1,px1\n", 10},
		{"zero block size", header, 0},
		{"negative block size", header, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewBlockReader(strings.NewReader(tt.input), tt.size); err == nil {
				t.Error("NewBlockReader() returned nil error, want an error")
			}
		})
	}
}
