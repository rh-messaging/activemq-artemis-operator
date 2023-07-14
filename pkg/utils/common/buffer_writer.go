package common

import "bytes"

type BufferWriter struct {
	Buffer *bytes.Buffer
}

func (w BufferWriter) Write(p []byte) (int, error) {
	if w.Buffer != nil {
		return w.Buffer.Write(p)
	} else {
		return len(p), nil
	}
}
