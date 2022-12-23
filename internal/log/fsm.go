package log

import (
	"bytes"
	"io"

	"github.com/hashicorp/raft"
	"google.golang.org/protobuf/proto"

	api "github.com/julieta-311/proglog/api/v1"
)

// fsm implements the raft.FSM interface
// which stands for finite-state machine,
// and is used to manage the data in the
// distributed log system.
type fsm struct {
	log *Log
}

var _ raft.FSM = (*fsm)(nil)

type RequestType uint8

const (
	AppendRequestType RequestType = 0
)

// Apply implements the Apply method of the raft.FSM
// interface and it's invoked after committing a log entry.
func (f *fsm) Apply(record *raft.Log) interface{} {
	buf := record.Data
	reqType := RequestType(buf[0])
	switch reqType {
	case AppendRequestType:
		return f.applyAppend(buf[1:])
	}

	return nil
}

func (f *fsm) applyAppend(b []byte) interface{} {
	var req api.ProduceRequest
	err := proto.Unmarshal(b, &req)
	if err != nil {
		return err
	}

	offset, err := f.log.Append(req.Record)
	if err != nil {
		return err
	}

	return &api.ProduceResponse{Offset: offset}
}

// Snapshot is periodically called by raft to
// snapshot its state.
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	r := f.log.Reader()
	return &snapshot{reader: r}, nil
}

// Restore is called by Raft to restore an FSM
// from a snapshot.
func (f *fsm) Restore(r io.ReadCloser) error {
	b := make([]byte, lenWidth)
	var buf bytes.Buffer
	for i := 0; ; i++ {
		_, err := io.ReadFull(r, b)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		size := int64(enc.Uint64(b))
		if _, err = io.CopyN(&buf, r, size); err != nil {
			return err
		}

		record := &api.Record{}
		if err = proto.Unmarshal(buf.Bytes(), record); err != nil {
			return err
		}

		if i == 0 {
			f.log.Config.Segment.InitialOffset = record.Offset
			if err := f.log.Reset(); err != nil {
				return err
			}
		}

		if _, err = f.log.Append(record); err != nil {
			return err
		}

		buf.Reset()
	}

	return nil
}
