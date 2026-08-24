package genieql

import (
	"fmt"
	"io"
)

// Generator interface for the code generators.
type Generator interface {
	Generate(dst io.Writer) error
}

// MultiGenerate generate multiple scanners into a single buffer.
func MultiGenerate(generators ...Generator) Generator {
	return multiGenerator{
		generators: generators,
	}
}

type multiGenerator struct {
	generators []Generator
}

func (t multiGenerator) Generate(dst io.Writer) error {
	for _, generator := range t.generators {
		if err := generator.Generate(dst); err != nil {
			return err
		}
		fmt.Fprintf(dst, "\n\n")
	}
	return nil
}

type copier struct {
	src io.Reader
}

func (t copier) Generate(dst io.Writer) error {
	_, err := io.Copy(dst, t.src)
	return err
}

// NewCopyGenerator simply copies the reader when generate is called.
func NewCopyGenerator(in io.Reader) Generator {
	return copier{src: in}
}

// NewErrGenerator builds a generate that errors out.
func NewErrGenerator(err error) Generator {
	return errGenerator{err: err}
}

type errGenerator struct {
	err error
}

func (t errGenerator) Generate(io.Writer) error {
	return t.err
}

type funcGenerator func(io.Writer) error

func (t funcGenerator) Generate(dst io.Writer) error {
	return t(dst)
}

// NewFuncGenerator pure function generator
func NewFuncGenerator(fn func(dst io.Writer) error) Generator {
	return funcGenerator(fn)
}
