package transformx

import (
	"bytes"
	"io"
	"log"

	"golang.org/x/text/transform"
)

// String converts s using the provided transformer.
// if an error occurs it returns the input unmodified.
func String(s string, m transform.Transformer) string {
	r, _, err := transform.String(
		m,
		s,
	)

	if err != nil {
		log.Println("unable to transform returning unmodified result", err)
		return s
	}

	return r
}

func Wrap(with string) transform.Transformer {
	return transform.Chain(Prefix(with), Suffix(with))
}

func Prefix(with string) transform.Transformer {
	return &prefix{atStart: true, with: []byte(with)}
}

func Suffix(with string) transform.Transformer {
	return &suffix{atStart: true, with: []byte(with)}
}

func Full(t func(string) string) transform.Transformer {
	return &full{
		m: t,
		b: bytes.NewBuffer(make([]byte, 1024)),
	}
}

type suffix struct {
	atStart bool
	with    []byte
}

func (t *suffix) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	nSrc = copy(dst[nDst:], src)
	nDst += nSrc

	if atEOF {
		nDst += copy(dst[nDst:], t.with)
	}

	return nDst, nSrc, nil
}

func (t *suffix) Reset() {
	t.atStart = true
}

type prefix struct {
	atStart bool
	with    []byte
}

func (t *prefix) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	if t.atStart {
		nDst += copy(dst, t.with)
		t.atStart = false
	}

	nSrc = copy(dst[nDst:], src)
	nDst += nSrc

	return nDst, nSrc, nil
}

func (t *prefix) Reset() {
	t.atStart = true
}

type full struct {
	m func(string) string
	b *bytes.Buffer
}

func (t *full) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	if n, err := io.Copy(t.b, bytes.NewReader(src)); err != nil {
		return nDst, int(n), err
	} else {
		nSrc = int(n)
	}

	if !atEOF {
		return nDst, nSrc, transform.ErrShortSrc
	}

	if len(dst) < t.b.Len() {
		return nDst, nSrc, transform.ErrShortDst
	}

	nDst = copy(dst, []byte(t.m(t.b.String())))

	return nDst, nSrc, err
}

func (t *full) Reset() {
	t.b.Reset()
}
