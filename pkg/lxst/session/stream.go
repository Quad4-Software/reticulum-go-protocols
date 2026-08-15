// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"encoding/binary"
	"fmt"
	stdio "io"
	"sync"

	"quad4/reticulum-go-protocols/pkg/lxst/audio/io"
)

func (s *Session) Attach(ctx context.Context, rw stdio.ReadWriter) error {
	if s.host == nil {
		return ErrNoHost
	}
	if rw == nil {
		return fmt.Errorf("missing stream")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errCh <- s.readCapture(ctx, rw)
	}()
	go func() {
		defer wg.Done()
		errCh <- s.writePlayback(ctx, rw)
	}()
	var first error
	select {
	case <-ctx.Done():
		first = ctx.Err()
	case first = <-errCh:
		cancel()
	}
	wg.Wait()
	return first
}

func (s *Session) readCapture(ctx context.Context, r stdio.Reader) error {
	hdr := make([]byte, 4)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := readFull(ctx, r, hdr); err != nil {
			return err
		}
		n := binary.LittleEndian.Uint32(hdr)
		if n == 0 || n > io.MaxPCMBytes {
			return fmt.Errorf("pcm frame length %d", n)
		}
		buf := make([]byte, n)
		if err := readFull(ctx, r, buf); err != nil {
			return err
		}
		if err := s.host.PushBytes(buf); err != nil {
			return err
		}
	}
}

func (s *Session) writePlayback(ctx context.Context, w stdio.Writer) error {
	hdr := make([]byte, 4)
	for {
		pcm, err := s.host.WaitPlayback(ctx)
		if err != nil {
			return err
		}
		raw := io.PCM16LE(pcm)
		binary.LittleEndian.PutUint32(hdr, uint32(len(raw))) // #nosec G115 -- pcm frames are capped by MaxPCMBytes
		if _, err := w.Write(hdr); err != nil {
			return err
		}
		if _, err := w.Write(raw); err != nil {
			return err
		}
	}
}

func readFull(ctx context.Context, r stdio.Reader, buf []byte) error {
	off := 0
	for off < len(buf) {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := r.Read(buf[off:])
		if n == 0 && err == nil {
			return stdio.ErrUnexpectedEOF
		}
		off += n
		if err != nil {
			return err
		}
	}
	return nil
}
