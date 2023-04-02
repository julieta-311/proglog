package log

import "github.com/hashicorp/raft"

// Config centralised a log's configuration.
type Config struct {
	Raft struct {
		raft.Config
		BindAddr    string
		StreamLayer *StreamLayer
		Bootstrap   bool
	}
	Segment struct {
		MaxStoreBytes uint64
		MaxIndexBytes uint64
		InitialOffset uint64
	}
}
