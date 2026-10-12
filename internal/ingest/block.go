package ingest

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
)

type Block struct {
	FirstLine int
	Data      []byte
}

type BlockReader struct {
	r        *bufio.Reader
	parser   Parser
	size     int // lines per block
	nextLine int
	hint     int // size of the previous block, to allocate the next one
}

func NewBlockReader(r io.Reader, linesPerBlock int) (*BlockReader, error) {
	if linesPerBlock < 1 {
		return nil, fmt.Errorf("ingest: lines per block must be positive, got %d", linesPerBlock)
	}

	br := &BlockReader{r: bufio.NewReaderSize(r, 64<<10), size: linesPerBlock, nextLine: 2}

	line, err := br.appendLine(nil)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("ingest: reading header: %w", err)
	}
	if len(line) == 0 {
		return nil, fmt.Errorf("ingest: reading header: %w", io.EOF)
	}

	fields, err := csv.NewReader(bytes.NewReader(line)).Read()
	if err != nil {
		return nil, fmt.Errorf("ingest: reading header: %w", err)
	}
	if br.parser, err = NewParser(fields); err != nil {
		return nil, err
	}
	return br, nil
}

// The error is nil, io.EOF (the input ended; the line may be
// there without a newline) or a real read error.
func (r *BlockReader) appendLine(dst []byte) ([]byte, error) {
	for {
		line, err := r.r.ReadSlice('\n')
		dst = append(dst, line...)
		if errors.Is(err, bufio.ErrBufferFull) {
			continue // a line longer than the buffer: keep reading it
		}
		return dst, err
	}
}

func (r *BlockReader) ReadBlock() (Block, error) {
	data := make([]byte, 0, r.hint)
	lines := 0

	for lines < r.size {
		before := len(data)
		var err error
		data, err = r.appendLine(data)
		if len(data) > before {
			lines++
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Block{}, err
		}
	}

	if lines == 0 {
		return Block{}, io.EOF
	}
	b := Block{FirstLine: r.nextLine, Data: data}
	r.nextLine += lines
	r.hint = len(data)
	return b, nil
}
