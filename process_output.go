package main

import (
	"bytes"
	"errors"
)

const maxCommandOutputBytes int64 = 64 << 20

var errCommandOutputTooLarge = errors.New("command output exceeds limit")

type limitedBuffer struct {
	buffer   bytes.Buffer
	maxBytes int64
	exceeded bool
}

func newLimitedBuffer(maxBytes int64) *limitedBuffer {
	return &limitedBuffer{maxBytes: maxBytes}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.maxBytes - int64(b.buffer.Len())
	if remaining <= 0 {
		b.exceeded = true
		return len(p), nil
	}
	if int64(len(p)) > remaining {
		_, _ = b.buffer.Write(p[:remaining])
		b.exceeded = true
		return len(p), nil
	}
	return b.buffer.Write(p)
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buffer.Bytes()
}

func (b *limitedBuffer) Exceeded() bool {
	return b.exceeded
}
