package mux

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
)

const (
	idOffset   = 0
	sizeOffset = idOffset + 4
	headerSize = sizeOffset + 4
)

type Writer struct {
	w  io.Writer
	id uint32
}

var _ io.Writer = (*Writer)(nil)

func NewWriter(w io.Writer, id uint32) *Writer {
	return &Writer{w: w, id: id}
}

func (w *Writer) write(b []byte) (int, error) {
	hdr := make([]byte, headerSize)
	binary.BigEndian.PutUint32(hdr[idOffset:sizeOffset], w.id)
	binary.BigEndian.PutUint32(hdr[sizeOffset:headerSize], uint32(len(b)))
	if _, err := w.w.Write(hdr); err != nil {
		return 0, err
	}
	return w.w.Write(b)
}

func (w *Writer) Write(b []byte) (int, error) {
	n := 0
	for len(b) > 0 {
		s := len(b)
		if s > math.MaxUint32 {
			s = math.MaxUint32
		}
		w, err := w.write(b[:s])
		n += w
		if err != nil {
			return n, err
		}
		b = b[s:]
	}
	return n, nil
}

type Receiver struct {
	o map[uint32]io.Writer
}

func NewReceiver() *Receiver {
	return &Receiver{o: make(map[uint32]io.Writer)}
}

func (rcv *Receiver) Add(id uint32, w io.Writer) {
	rcv.o[id] = w
}

func (rcv *Receiver) SplitCopy(r io.Reader) error {
	buffer := make([]byte, 4096)
	chunkComplete := true
	chunkSize := 0
	id := uint32(0)
	hdr := make([]byte, 0, headerSize)
	msg := make([]byte, 0, 4096)

	var buf []byte
	for {
		if len(buf) == 0 {
			buf = buffer[:]
			n, err := r.Read(buf)
			if errors.Is(err, io.EOF) {
				if chunkComplete {
					return nil
				}
				return io.ErrUnexpectedEOF
			}
			if err != nil {
				return err
			}
			if n == 0 {
				continue
			}
			buf = buf[:n]
		}

		if chunkComplete {
			chunkComplete = false

			// start of new header
			if len(buf) < headerSize {
				hdr = append(hdr, buf...)
				continue
			}

			hdr = append(hdr, buf[:headerSize]...)
			buf = buf[headerSize:]

			id = binary.BigEndian.Uint32(hdr[idOffset:sizeOffset])
			chunkSize = int(binary.BigEndian.Uint32(hdr[sizeOffset:headerSize]))

			hdr = hdr[:0]
			msg = msg[:0]
		}

		left := chunkSize - len(msg)
		if len(buf) < left {
			msg = append(msg, buf...)
			continue
		}

		chunkComplete = true
		msg = append(msg, buf[:left]...)
		buf = buf[left:]

		if w := rcv.o[id]; w != nil {
			if _, err := w.Write(msg); err != nil {
				return err
			}
		}
	}
}
