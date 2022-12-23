package log

import (
	"github.com/hashicorp/raft"

	api "github.com/julieta-311/proglog/api/v1"
)

var _ raft.LogStore = (*logStore)(nil)

type logStore struct {
	*Log
}

func newLogStore(dir string, c Config) (*logStore, error) {
	log, err := NewLog(dir, c)
	if err != nil {
		return nil, err
	}

	return &logStore{log}, nil
}

// FirstIndex implements the FirstIndex method of
// the raft.LogStore interface, and returns the first
// index written or 0 for no entries.
func (l *logStore) FirstIndex() (uint64, error) {
	return l.LowestOffset()
}

// LastIndex implements the LastIndex method of
// the raft.LogStore interface, and returns the last
// index written or 0 for no entries.
func (l *logStore) LastIndex() (uint64, error) {
	off, err := l.HighestOffset()
	return off, err
}

// GetLog implements the GetLog method of the
// raft.LogStore interface to get a log at a given index.
func (l *logStore) GetLog(index uint64, out *raft.Log) error {
	in, err := l.Read(index)
	if err != nil {
		return err
	}

	out.Data = in.Value
	out.Index = in.Offset
	out.Type = raft.LogType(in.Type)
	out.Term = in.Term

	return nil
}

// StoreLog implements the method of the same name in
// the raft.LogStore interface which is used to store
// a given log entry.
func (l *logStore) StoreLog(record *raft.Log) error {
	return l.StoreLogs([]*raft.Log{record})
}

// StoreLogs implements the method of the same name in
// the raft.LogStore interface which is used to store
// multiple log entries.
func (l *logStore) StoreLogs(records []*raft.Log) error {
	for _, record := range records {
		if _, err := l.Append(&api.Record{
			Value: record.Data,
			Term:  record.Term,
			Type:  uint32(record.Type),
		}); err != nil {
			return err
		}
	}

	return nil
}

// DeleteRange removes the records between the given
// offsets. It is used by raft to remove old records
// or records stored in a snapshot.
func (l *logStore) DeleteRange(min, max uint64) error {
	return l.Truncate(max)
}
