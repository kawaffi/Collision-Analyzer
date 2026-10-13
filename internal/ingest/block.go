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

func (r *BlockReader) Parser() Parser {
	return r.parser
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

// ParseBlock parses the records in b and calls yield, in order, for every
// valid event. Malformed rows are skipped and counted; the first one is also
// returned as firstErr (wrapping ErrMalformedRow, with its line number in the
// input) so that callers can show where the problem is. A Parser is immutable
// and ParseBlock keeps no state, so it can run on several goroutines at once.
func (p Parser) ParseBlock(b Block, yield func(Event)) (skipped int, firstErr error) {
	cr := csv.NewReader(bytes.NewReader(b.Data))
	cr.FieldsPerRecord = -1

	skip := func(err error) {
		skipped++
		if firstErr == nil {
			firstErr = err
		}
	}

	for {
		fields, err := cr.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return skipped, firstErr
			}
			var parseErr *csv.ParseError
			if errors.As(err, &parseErr) {
				skip(fmt.Errorf("%w: line %d: %v", ErrMalformedRow, b.FirstLine+parseErr.StartLine-1, parseErr.Err))
				continue
			}
			skip(fmt.Errorf("%w: %v", ErrMalformedRow, err))
			return skipped, firstErr
		}

		line, _ := cr.FieldPos(0)
		ev, err := p.Parse(Record{Line: b.FirstLine + line - 1, Fields: fields})
		if err != nil {
			skip(err)
			continue
		}
		yield(ev)
	}
}
