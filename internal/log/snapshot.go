package log

import (
	"io"

	"github.com/hashicorp/raft"
)

var _ raft.FSMSnapshot = (*snapshot)(nil)

type snapshot struct {
	reader io.Reader
}

// Persist is called by Raft on the FSMSnapshot to
// write its state to the sink.
func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	if _, err := io.Copy(sink, s.reader); err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

// Release is called by Raft when it's
// finished with a snapshot.
func (s *snapshot) Release() {}
