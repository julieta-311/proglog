package server

import (
	"fmt"
	"sync"
)

// Record is an value logged at a given offset.
type Record struct {
	Value  []byte `json:"value"`
	Offset uint64 `json:"offset"`
}

// ErrOffsetNotFound is an error returned if there are
// no records at the given offset.
var ErrOffsetNotFound = fmt.Errorf("offset not found")

// Log holds the records logged.
type Log struct {
	mu      sync.Mutex
	records []Record
}

// NewLog instanciates a Log.
func NewLog() *Log {
	return &Log{}
}

// Append adds a new record to the given Log.
func (c *Log) Append(record Record) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	record.Offset = uint64(len(c.records))
	c.records = append(c.records, record)

	return record.Offset, nil
}

// Read reads the record at the given offset in the Log.
func (c *Log) Read(offset uint64) (Record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if offset >= uint64(len(c.records)) {
		return Record{}, ErrOffsetNotFound
	}

	return c.records[offset], nil
}
