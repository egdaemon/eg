package stringsx

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
)

// DefaultIfBlank uses the provided default value if s is blank.
func DefaultIfBlank(s, defaultValue string) string {
	if len(strings.TrimSpace(s)) == 0 {
		return defaultValue
	}
	return s
}

// Contains returns true iff s matches one of the strings in v
func Contains(s string, v ...string) bool {
	for _, x := range v {
		if s == x {
			return true
		}
	}
	return false
}

// ToPrivate lowercases the first letter.
func ToPrivate(s string) string {
	if s == "" {
		return ""
	}

	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// ToPublic ...
func ToPublic(s string) string {
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// Debug converts unprintable characters to their unicode character sequence.
func Debug(i string) (string, error) {
	check := func(r rune, set ...rune) bool {
		for _, c := range set {
			if r == c {
				return true
			}
		}

		return false
	}

	buf := bytes.NewBufferString("")
	for _, r := range []rune(i) {
		convert := !unicode.IsPrint(r)
		convert = convert && !check(r, '\r', '\n', '\t')

		if convert {
			if _, err := buf.WriteString(fmt.Sprintf("%U", []rune{r})); err != nil {
				return "", err
			}
			continue
		}

		if _, err := buf.WriteRune(r); err != nil {
			return "", err
		}

	}

	return buf.String(), nil
}

// DebugString returns the debug string unless there is an error,
// then it returns the original string
func DebugString(s string) string {
	if r, err := Debug(s); err == nil {
		return r
	}

	return s
}

// Reverse returns the string reversed rune-wise left to right.
func Reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
