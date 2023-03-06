package mux

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"math"
	"sync"
)

const (
	idOffset   = 0
	sizeOffset = idOffset + 4
	headerSize = sizeOffset + 4
)

type Writer struct {
	w    io.Writer
	id   uint32
	m    sync.Mutex
	resp chan error
}

var _ io.Writer = (*Writer)(nil)

func newWriter(w io.Writer, id uint32) *Writer {
	return &Writer{w: w, id: id, resp: make(chan error, 1)}
}

func (w *Writer) write(b []byte) (int, error) {
	w.m.Lock()
	defer w.m.Unlock()
	if w.w == nil {
		return 0, io.ErrClosedPipe
	}
	hdr := make([]byte, headerSize)
	binary.BigEndian.PutUint32(hdr[idOffset:sizeOffset], w.id)
	binary.BigEndian.PutUint32(hdr[sizeOffset:headerSize], uint32(len(b)))
	if _, err := w.w.Write(hdr); err != nil {
		return 0, err
	}
	if _, err := w.w.Write(b); err != nil {
		return 0, err
	}
	if err := <-w.resp; err != nil {
		return 0, err
	}
	return len(b), nil
}

func (w *Writer) response(err error) {
	w.resp <- err
}

func (w *Writer) Write(b []byte) (int, error) {
	n := 0
	for len(b) > 0 {
		s := len(b)
		if s > math.MaxInt32 {
			s = math.MaxInt32
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

func (w *Writer) Close() error {
	// TODO: close should remove from receiver
	w.m.Lock()
	defer w.m.Unlock()
	if w.w == nil {
		// already closed
		return nil
	}
	out := w.w
	w.w = nil
	// we send a header with size == 0 to close the connection
	hdr := make([]byte, headerSize)
	binary.BigEndian.PutUint32(hdr[idOffset:sizeOffset], w.id)
	binary.BigEndian.PutUint32(hdr[sizeOffset:headerSize], 0)
	if _, err := out.Write(hdr); err != nil {
		return err
	}
	return nil
}

type receiver struct {
	o map[uint32]io.Writer
	w map[uint32]*Writer
	m *Mux
}

func newReceiver(m *Mux) *receiver {
	return &receiver{
		o: make(map[uint32]io.Writer),
		w: make(map[uint32]*Writer),
		m: m,
	}
}

func (rcv *receiver) Add(id uint32, w io.Writer) {
	rcv.o[id] = w
}

func (rcv *receiver) addWriter(id uint32, w *Writer) {
	rcv.w[id] = w
}

func (rcv *receiver) Close(id uint32) {
	delete(rcv.o, id)
	delete(rcv.w, id)
}

func (rcv *receiver) SplitCopy(name string, r io.Reader) (err error) {
	buffer := make([]byte, 4096)
	chunkComplete := true
	chunkSize := 0
	isError := false
	id := uint32(0)
	hdr := make([]byte, 0, headerSize)
	msg := make([]byte, 0, 4096)

	// log.Printf("RECV: %v %v", rcv.o, rcv.w)
	defer func() {
		log.Printf("RECV DONE: %v %v %s", rcv.o, rcv.w, err)
	}()

	var buf []byte
	for {
		// log.Printf("LOOP(%p): %d %v %d", rcv, len(buf), chunkComplete, chunkSize)
		if len(buf) == 0 {
			buf = buffer[:]
			n, err := r.Read(buf)
			log.Printf("READ(%s): %d %v %s", name, n, buf[:n], err)
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
			isError = false

			// log.Printf("READ HDR")

			// start of new header
			if len(buf) < headerSize {
				hdr = append(hdr, buf...)
				continue
			}

			hdr = append(hdr, buf[:headerSize]...)
			buf = buf[headerSize:]

			id = binary.BigEndian.Uint32(hdr[idOffset:sizeOffset])
			chunkSize = int(binary.BigEndian.Uint32(hdr[sizeOffset:headerSize]))

			// log.Printf("HDR: %d %d", id, chunkSize)

			if chunkSize&0x80000000 != 0 {
				chunkSize &= ^0x80000000
				isError = true
				// log.Printf("ERROR: %d %d", id, chunkSize)
			}

			hdr = hdr[:0]
			msg = msg[:0]
		}

		if chunkSize == 0 {
			chunkComplete = true
			if isError {
				// we need to report no error to writer
				// log.Printf("SUCCESS: %d %p", id, rcv.w[id])
				if w := rcv.w[id]; w != nil {
					// log.Printf("RESPONSE: %d %p", id, w)
					w.response(nil)
				}
				continue
			}
			// this is a close
			if w := rcv.o[id]; w != nil {
				// log.Printf("CLOSE: %d", id)
				if c, ok := w.(io.Closer); ok {
					c.Close()
				}
				delete(rcv.o, id)
			}
			continue
		}

		left := chunkSize - len(msg)
		// log.Printf("LEFT(%d): %d %d", id, len(buf), left)
		if len(buf) < left {
			msg = append(msg, buf...)
			continue
		}

		// log.Printf("COMPLETE: %d", id)
		chunkComplete = true
		msg = append(msg, buf[:left]...)
		buf = buf[left:]

		if isError {
			// report error to the writer
			// log.Printf("ERROR: %d '%s' %p", id, msg, rcv.w[id])
			if w := rcv.w[id]; w != nil {
				// log.Printf("RESPONSE: %d %p", id, w)
				w.response(errors.New(string(msg)))
			}
			continue
		}

		// log.Printf("MSG(%d): %p %v", id, rcv.o[id], msg)
		if w := rcv.o[id]; w != nil {
			// log.Printf("WRITE(%d): %v", id, msg)
			_, err := w.Write(msg)
			// log.Printf("WRITE %d RESULT: %s", id, err)
			if rcv.m != nil {
				// log.Printf("SEND ERROR: %d %s", id, err)
				rcv.m.sendError(id, err)
			}
			// delete(rcv.o, id)
		}
	}
}

type Conn struct {
	w *Writer
	r *io.PipeReader
}

var _ io.ReadWriteCloser = (*Conn)(nil)

func (c *Conn) Write(b []byte) (int, error) {
	return c.w.Write(b)
}

func (c *Conn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

func (c *Conn) Close() error {
	c.r.Close()
	return c.w.Close()
}

type Mux struct {
	out  io.Writer
	in   io.Reader
	r    *receiver
	name string
}

func New(name string, in io.Reader, out io.Writer) *Mux {
	m := &Mux{
		out:  out,
		in:   in,
		name: name,
	}
	m.r = newReceiver(m)
	return m
}

func (m *Mux) Connect(id uint32) *Conn {
	return &Conn{
		w: m.Send(id),
		r: m.Read(id),
	}
}

func (m *Mux) write(id uint32, data []byte, flag bool) error {
	msg := make([]byte, headerSize+len(data))
	chunkSize := uint32(len(data))
	if flag {
		chunkSize |= uint32(0x80000000)
	}
	binary.BigEndian.PutUint32(msg[idOffset:sizeOffset], id)
	binary.BigEndian.PutUint32(msg[sizeOffset:headerSize], chunkSize)
	copy(msg[headerSize:], data)
	// log.Printf("MUX WRITE: %d %d %d %v", len(data), chunkSize, len(msg), msg)
	if _, err := m.out.Write(msg); err != nil {
		return err
	}
	return nil
}

func (m *Mux) sendError(id uint32, err error) error {
	if err == nil {
		return m.write(id, nil, true)
	}
	return m.write(id, []byte(err.Error()), true)
}

func (m *Mux) Send(id uint32) *Writer {
	w := newWriter(m.out, id)
	m.r.addWriter(id, w)
	return w
}

func (m *Mux) Recv(id uint32, w io.Writer) {
	m.r.Add(id, w)
}

func (m *Mux) Read(id uint32) *io.PipeReader {
	pr, pw := io.Pipe()
	m.r.Add(id, pw)
	return pr
}

func (m *Mux) Process() error {
	return m.r.SplitCopy(m.name, m.in)
}

func (m *Mux) Close() error {
	if c, ok := m.in.(io.Closer); ok {
		if err := c.Close(); err != nil {
			return err
		}
	}
	if c, ok := m.out.(io.Closer); ok {
		if err := c.Close(); err != nil {
			return err
		}
	}
	for _, w := range m.r.o {
		if c, ok := w.(io.Closer); ok {
			c.Close()
		}
	}
	for _, w := range m.r.w {
		w.Close()
	}
	return nil
}
