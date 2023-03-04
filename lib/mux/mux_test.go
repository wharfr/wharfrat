package mux_test

import (
	"io"
	"log"
	"testing"

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

	in = mux.New(ir, ow)
	out = mux.New(or, iw)

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
