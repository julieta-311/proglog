package log

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	api "github.com/julieta-311/proglog/api/v1"
)

func TestLog(t *testing.T) {
	for scenario, fn := range map[string]func(
		t *testing.T,
		log *Log,
	){
		"append and read a record succeeds": testAppendRead,
		"offset out of range":               testOutOfRangeErr,
		"init with existing segments":       testInitExisting,
		"reader":                            testReader,
		"truncate":                          testTruncate,
	} {
		t.Run(scenario, func(s *testing.T) {
			dir, err := os.MkdirTemp("", "store-test")
			require.NoError(s, err)
			defer os.RemoveAll(dir)

			c := Config{}
			c.Segment.MaxStoreBytes = 32
			log, err := NewLog(dir, c)
			require.NoError(s, err)

			fn(s, log)
		})
	}
}

func testAppendRead(t *testing.T, log *Log) {
	append := &api.Record{
		Value: []byte("Hallo Welt"),
	}
	off, err := log.Append(append)
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	read, err := log.Read(off)
	require.NoError(t, err)
	require.Equal(t, append.Value, read.Value)
}

func testOutOfRangeErr(t *testing.T, log *Log) {
	read, err := log.Read(1)
	require.Nil(t, read)
	apiErr := err.(api.ErrOffsetOutOfRange)
	require.Equal(t, uint64(1), apiErr.Offset)
}

func testInitExisting(t *testing.T, o *Log) {
	append := &api.Record{
		Value: []byte("Bonjour le monde"),
	}
	for i := 0; i < 3; i++ {
		_, err := o.Append(append)
		require.NoError(t, err)
	}
	require.NoError(t, o.Close())

	off, err := o.LowestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	off, err = o.HighestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(2), off)

	n, err := NewLog(o.Dir, o.Config)
	require.NoError(t, err)

	off, err = n.LowestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	off, err = n.HighestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(2), off)
}

func testReader(t *testing.T, log *Log) {
	append := &api.Record{
		Value: []byte("Olá Mundo"),
	}
	off, err := log.Append(append)
	require.NoError(t, err)
	require.Equal(t, uint64(0), off)

	reader := log.Reader()
	b, err := io.ReadAll(reader)
	require.NoError(t, err)

	read := &api.Record{}
	err = proto.Unmarshal(b[lenWidth:], read)
	require.NoError(t, err)
	require.Equal(t, append.Value, read.Value)
}

func testTruncate(t *testing.T, log *Log) {
	for i := 0; i < 3; i++ {
		append := &api.Record{
			Value: []byte(fmt.Sprintf("Hej världen %d", i)),
		}
		_, err := log.Append(append)
		require.NoError(t, err)
	}

	err := log.Truncate(1)
	require.NoError(t, err)

	lowestOff, err := log.LowestOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(2), lowestOff)

	b, err := log.Read(2) // should be 0?
	require.NoError(t, err)
	require.Equal(t, "Hej världen 2", string(b.Value))
}

func TestNewLogSetsDefaultConfig(t *testing.T) {
	dir, err := os.MkdirTemp("", "test-new-log-sets-defaults")
	require.NoError(t, err)

	l, err := NewLog(dir, Config{})
	require.NoError(t, err)

	want := Config{}
	want.Segment.MaxStoreBytes = 1024
	want.Segment.MaxIndexBytes = 1024
	want.Segment.InitialOffset = 0
	require.Equal(t, l.Config, want)
}
