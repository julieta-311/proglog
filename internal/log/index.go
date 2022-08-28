package log

import (
	"fmt"
	"io"
	"os"

	"github.com/tysonmote/gommap"
)

var (
	offWidth uint64 = 4
	posWidth uint64 = 8
	entWidth        = offWidth + posWidth
)

// index defines the index file thaat comprises a persisted file
// and a memory mapped file. The size is the size of the index
// and indicates where to write a new entry appended to the index.
type index struct {
	file *os.File
	mmap gommap.MMap
	size uint64
}

func newIndex(f *os.File, c Config) (*index, error) {
	idx := &index{
		file: f,
	}
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to get file stat: %w", err)
	}

	// Since memory mapped files cannot be resized, grow the
	// file to the max index size before memmory-mapping it.
	idx.size = uint64(fi.Size())
	if err = os.Truncate(
		f.Name(),
		int64(c.Segment.MaxIndexBytes),
	); err != nil {
		return nil, fmt.Errorf("failed to truncate file: %w", err)
	}

	if idx.mmap, err = gommap.Map(
		idx.file.Fd(),
		gommap.PROT_READ|gommap.PROT_WRITE,
		gommap.MAP_SHARED,
	); err != nil {
		return nil, fmt.Errorf("failed to memory map index file: %w", err)
	}

	return idx, nil
}

// Close makes sure the memory mapped file has synced its data to
// the persisted file and that the persisted file has flushed and
// that the persisted file has flushed its contents to stable
// storage before truncating the persisted file to the amount of
// data in the file and closing the file.
func (i *index) Close() error {
	if err := i.mmap.Sync(gommap.MS_SYNC); err != nil {
		return err
	}

	if err := i.file.Sync(); err != nil {
		return err
	}

	if err := i.file.Truncate(int64(i.size)); err != nil {
		return err
	}

	return i.file.Close()
}

// Read takes in an offset and returns the associated record's position
// in the store. 0 is always the offset of the index's first entry.
func (i *index) Read(in int64) (out uint32, pos uint64, err error) {
	if i.size == 0 {
		return 0, 0, io.EOF
	}

	if in == -1 {
		out = uint32((i.size / entWidth) - 1)
	} else {
		out = uint32(in)
	}

	pos = uint64(out) * entWidth
	if i.size < pos+entWidth {
		return 0, 0, io.EOF
	}

	out = enc.Uint32(i.mmap[pos : pos+offWidth])
	pos = enc.Uint64(i.mmap[pos+offWidth : pos+entWidth])
	return out, pos, nil
}

// Write append the given offset and position to the index.
func (i *index) Write(off uint32, pos uint64) error {
	if uint64(len(i.mmap)) < i.size+entWidth {
		return io.EOF
	}

	enc.PutUint32(i.mmap[i.size:i.size+offWidth], off)
	enc.PutUint64(i.mmap[i.size+offWidth:i.size+entWidth], pos)
	i.size += uint64(entWidth)

	return nil
}

// Name returns the name of the index file.
func (i *index) Name() string {
	return i.file.Name()
}
