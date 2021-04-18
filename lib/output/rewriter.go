package output

import (
	"bytes"
	"io"
	"log"
)

type Rewriter struct {
	w              io.Writer
	buf1, buf2     bytes.Buffer
	match, replace []byte
}

var (
	_ io.WriteCloser = (*Rewriter)(nil)
)

func NewRewriter(w io.Writer, match, replace []byte) io.Writer {
	if len(match) == 0 {
		return w
	}

	return &Rewriter{
		w:       w,
		match:   match,
		replace: replace,
	}
}

func slice(s []byte, idx, length int) []byte {
	if length > len(s[idx:]) {
		length = len(s[idx:])
	}
	return s[idx:idx+length]
}

func (r *Rewriter) checkAndReplace() {
start:
	data := r.buf1.Bytes()

	i := 0
	for ; i < len(data) && !bytes.HasPrefix(r.match, slice(data, i, len(r.match))); i++ {
	}

	log.Printf("SKIP: %d", i)

	if i > 0 {
		// push unwanted bytes into buf2
		r.buf2.Write(r.buf1.Next(i))
	}

	if bytes.HasPrefix(r.buf1.Bytes(), r.match) {
		log.Printf("REPLACE: %s -> %s", r.match, r.replace)
		r.buf2.Write(r.replace)
		r.buf1.Next(len(r.match))
		goto start
	}
}

func (r *Rewriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	log.Printf("WRITE: %s", p)

	if n, err := r.buf1.Write(p); err != nil {
		return n, err
	}

	r.checkAndReplace()

	if r.buf2.Len() > 0 {
		if _, err := r.buf2.WriteTo(r.w); err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

func (r *Rewriter) Close() error {
	if r.buf1.Len() > 0 {
		r.buf1.WriteTo(&r.buf2)
	}

	if r.buf2.Len() > 0 {
		if _, err := r.buf2.WriteTo(r.w); err != nil {
			return err
		}
	}

	if c, ok := r.w.(io.Closer); ok {
		return c.Close()
	}

	return nil
}
