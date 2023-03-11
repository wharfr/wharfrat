package mux_test

import (
	"io"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"wharfr.at/wharfrat/lib/mux"
)

type output struct {
	out []byte
	err error
}

func setup() (in, out *mux.Mux) {
	ir, iw := io.Pipe()
	or, ow := io.Pipe()

	in = mux.New("in", ir, ow)
	out = mux.New("out", or, iw)

	return
}

func transmit(msg string, in io.Writer, out io.Reader) (string, error) {
	resp := make(chan error, 1)
	go func() {
		_, err := in.Write([]byte(msg))
		resp <- err
	}()
	buf := make([]byte, len(msg))
	if _, err := out.Read(buf); err != nil {
		return "", err
	}
	if err := <-resp; err != nil {
		return "", err
	}
	return string(buf), nil
}

func TestConn(t *testing.T) {
	assert := assert.New(t)

	in, out := setup()
	defer in.Close()
	defer out.Close()

	respIn := make(chan error, 1)
	go func() {
		respIn <- in.Process()
	}()

	respOut := make(chan error, 1)
	go func() {
		respOut <- out.Process()
	}()

	cIn := in.Connect(0)
	defer cIn.Close()
	cOut := out.Connect(0)
	defer cOut.Close()

	output, err := transmit("hello", cIn, cOut)

	assert.Nil(err)
	assert.Equal("hello", output)

	output, err = transmit("another message", cIn, cOut)

	assert.Nil(err)
	assert.Equal("another message", output)
}

func TestConnCloseOut(t *testing.T) {
	assert := assert.New(t)

	in, out := setup()
	defer in.Close()
	defer out.Close()

	respIn := make(chan error, 1)
	go func() {
		respIn <- in.Process()
		log.Printf("IN DONE")
	}()

	respOut := make(chan error, 1)
	go func() {
		respOut <- out.Process()
		log.Printf("OUT DONE")
	}()

	cIn := in.Connect(0)
	cOut := out.Connect(0)

	err := cOut.Close()
	assert.Nil(err)

	n, err := cIn.Write([]byte("hello"))

	assert.Equal(0, n)
	assert.Error(err)
}

func TestConnCloseIn(t *testing.T) {
	assert := assert.New(t)

	in, out := setup()
	defer in.Close()
	defer out.Close()

	respIn := make(chan error, 1)
	go func() {
		respIn <- in.Process()
	}()

	respOut := make(chan error, 1)
	go func() {
		respOut <- out.Process()
	}()

	cIn := in.Connect(0)
	cOut := out.Connect(0)

	err := cIn.Close()
	assert.Nil(err)

	buf := make([]byte, 1024)
	n, err := cOut.Read(buf)

	assert.Equal(0, n)
	assert.Equal(io.EOF, err)
}

func TestConnCloseWrite(t *testing.T) {
	assert := assert.New(t)

	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)

	in, out := setup()
	defer in.Close()
	defer out.Close()

	respIn := make(chan error, 1)
	go func() {
		respIn <- in.Process()
	}()

	respOut := make(chan error, 1)
	go func() {
		respOut <- out.Process()
	}()

	log.Printf("CONNECT")
	cIn := in.Connect(0)
	cOut := out.Connect(0)

	log.Printf("CLOSE WRITE")
	err := cIn.CloseWrite()
	assert.Nil(err)

	log.Printf("READ OUT")
	buf := make([]byte, 1024)
	n, err := cOut.Read(buf)

	assert.Equal(0, n)
	assert.Equal(io.EOF, err)

	log.Printf("WRITE")
	writeErr := make(chan error, 1)
	go func() {
		_, err := cOut.Write([]byte("HELLO"))
		writeErr <- err
	}()

	log.Printf("READ IN")
	n, err = cIn.Read(buf)

	assert.Equal(5, n)
	assert.Nil(err)

	select {
	case <-time.After(10 * time.Millisecond):
		assert.Fail("")
	case err := <-writeErr:
		assert.Nil(err)
	}

	log.Printf("DONE")
}

func TestConnCloseRead(t *testing.T) {
	assert := assert.New(t)

	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)

	in, out := setup()
	defer in.Close()
	defer out.Close()

	respIn := make(chan error, 1)
	go func() {
		respIn <- in.Process()
	}()

	respOut := make(chan error, 1)
	go func() {
		respOut <- out.Process()
	}()

	log.Printf("CONNECT")
	cIn := in.Connect(0)
	cOut := out.Connect(0)

	log.Printf("CLOSE READ")
	err := cIn.CloseRead()
	assert.Nil(err)

	log.Printf("WRITE OUT")
	buf := make([]byte, 1024)
	n, err := cOut.Write([]byte("HELLO"))

	assert.Equal(0, n)
	assert.Equal(io.ErrClosedPipe, err)

	log.Printf("WRITE IN")
	writeErr := make(chan error, 1)
	go func() {
		_, err := cIn.Write([]byte("HELLO"))
		writeErr <- err
	}()

	log.Printf("READ OUT")
	n, err = cOut.Read(buf)

	assert.Equal(5, n)
	assert.Nil(err)

	select {
	case <-time.After(10 * time.Millisecond):
		assert.Fail("")
	case err := <-writeErr:
		assert.Nil(err)
	}

	log.Printf("DONE")
}

func TestMuxClose(t *testing.T) {
	assert := assert.New(t)

	in, out := setup()

	pCh := make(chan error, 1)
	go func() {
		pCh <- in.Process()
	}()

	err := out.Close()
	assert.Nil(err)

	const timeout = 10 * time.Millisecond
	select {
	case err := <-pCh:
		assert.Nil(err)
	case <-time.After(timeout):
		assert.Fail("Process failed to return in %s", timeout)
	}

	assert.Fail("...")
}
