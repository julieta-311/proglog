package log

import (
	"bufio"
	"encoding/binary"
	"os"
	"sync"
)

// enc is the encoding that record sizes and index entries are persisted in.
var enc = binary.BigEndian

// lenWidth is the number of bytes used to store a record's length.
const lenWidth = 8

// store is a wrapper around a file with two APIs to append and read bytes to
// and from the file.
type store struct {
	*os.File
	mu   sync.Mutex
	buf  *bufio.Writer
	size uint64
}

func newStore(f *os.File) (*store, error) {
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}

	return &store{
		File: f,
		size: uint64(fi.Size()),
		buf:  bufio.NewWriter(f),
	}, nil

}

// Append persists the given bytes to the store, and writes the length of the
// record so that, when reading the record, the amount of bytes to read is known.
func (s *store) Append(p []byte) (n uint64, pos uint64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pos = s.size

	if err := binary.Write(s.buf, enc, uint64(len(p))); err != nil {
		return 0, 0, err
	}

	w, err := s.buf.Write(p)
	if err != nil {
		return 0, 0, err
	}

	w += lenWidth
	s.size += uint64(w)

	return uint64(w), pos, nil
}

// Read returns the record store at the given position.
func (s *store) Read(pos uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Flush the writer buffer in case the reacord to be read hasn't
	// been flushed to disk yet.
	if err := s.buf.Flush(); err != nil {
		return nil, err
	}

	size := make([]byte, lenWidth)
	if _, err := s.File.ReadAt(size, int64(pos)); err != nil {
		return nil, err
	}

	b := make([]byte, enc.Uint64(size))
	if _, err := s.File.ReadAt(b, int64(pos+lenWidth)); err != nil {
		return nil, err
	}

	return b, nil
}

// ReadAt implements io.ReadAt for the store type. It reads len(p) bytes
// into p beginning at the off offset in the store's file.
func (s *store) ReadAt(p []byte, off int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.buf.Flush(); err != nil {
		return 0, err
	}

	return s.File.ReadAt(p, off)
}

// Close persists any buffered data before closing the file.
func (s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.buf.Flush(); err != nil {
		return err
	}

	return s.File.Close()
}
