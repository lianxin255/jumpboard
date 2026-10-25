package services

import (
	"bytes"
	"fmt"
	"os"
)

// tailChunk is how much of a file is read at a time, from the end.
const tailChunk = 32 << 10

// TailFile returns the last n lines of a file.
//
// It reads backwards in blocks rather than the whole file, so that a
// multi-gigabyte access log costs no more than a small one.
func TailFile(path string, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	// A directory where a log file should be is a broken engine layout,
	// not an empty log. On Windows a directory's Size is 0, so treating
	// size==0 as empty would hide the error the log page must show.
	if st.IsDir() {
		return nil, fmt.Errorf("%s 是目录，不是日志文件", path)
	}
	size := st.Size()
	if size == 0 {
		return nil, nil
	}

	var (
		data     []byte
		offset   = size
		newlines int
	)
	for offset > 0 && newlines <= n {
		readSize := int64(tailChunk)
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize
		buf := make([]byte, readSize)
		if _, err := f.ReadAt(buf, offset); err != nil {
			return nil, err
		}
		data = append(buf, data...)
		newlines = bytes.Count(data, []byte{'\n'})
	}

	lines := bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'})
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = string(l)
	}
	return out, nil
}
