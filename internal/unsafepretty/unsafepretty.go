package unsafepretty

import (
	"fmt"
	"unicode"
)

type option func(*unsafepretty)

type unsafepretty struct {
	Newline []rune
	Space   []rune
}

func OptionNewlineRunes(runes ...rune) option {
	return func(c *unsafepretty) {
		c.Newline = runes
	}
}

func OptionSpaceRunes(runes ...rune) option {
	return func(c *unsafepretty) {
		c.Space = runes
	}
}

func OptionDisplaySpaces() option {
	return OptionSpaceRunes('·')
}

// Print makes whitespace characters visible when Printing
// method is unsafe because it will panic on error.
func Print(in string, options ...option) string {
	config := unsafepretty{
		Newline: []rune{'\n'},
		Space:   []rune{' '},
	}

	for _, opt := range options {
		opt(&config)
	}

	s := []rune(in)
	o := make([]rune, 0, len(s))
	for _, r := range s {
		if unicode.IsSpace(r) {
			var c []rune
			switch r {
			case '\n':
				c = config.Newline
			case '\r':
				c = []rune{'\\', 'r'}
			case '\t':
				c = []rune{'\\', 't'}
			case ' ':
				c = config.Space
			}

			o = append(o, c...)
			continue
		}

		if !unicode.IsPrint(r) {
			o = append(o, []rune(fmt.Sprintf("%U", r))...)
			continue
		}
		o = append(o, r)
	}

	return string(o)
}
