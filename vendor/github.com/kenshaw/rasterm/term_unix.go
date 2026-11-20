//go:build unix

package rasterm

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"golang.org/x/term"
)

// hasSixelSupport returns true if sixel support is available.
func hasSixelSupport() bool {
	attrs, err := termAttributes(os.Stdin, os.Stdout)
	if err != nil {
		return false
	}
	for i := range attrs {
		// ignore `4` @ 1st index -- that is terminal id rather than sixel support
		if 0 < i && attrs[i] == 4 {
			return true
		}
	}
	return false
}

// termAttributes requests terminal attributes.
//
//	CSI Ps c  Send Device Attributes (Primary DA).
//		Ps = 0  or omitted ⇒  request attributes from terminal.  The
//	response depends on the decTerminalID resource setting.
//		⇒  CSI ? 1 ; 2 c     ("VT100 with Advanced Video Option")
//		⇒  CSI ? 1 ; 0 c     ("VT101 with No Options")
//		⇒  CSI ? 4 ; 6 c     ("VT132 with Advanced Video and Graphics")
//		⇒  CSI ? 6 c         ("VT102")
//		⇒  CSI ? 7 c         ("VT131")
//		⇒  CSI ? 1 2 ; Ps c  ("VT125")
//		⇒  CSI ? 6 2 ; Ps c  ("VT220")
//		⇒  CSI ? 6 3 ; Ps c  ("VT320")
//		⇒  CSI ? 6 4 ; Ps c  ("VT420")
//
// The VT100-style response parameters do not mean anything by themselves.
// VT220 (and higher) parameters do, telling the host what features the
// terminal supports:
//
//	Ps = 1    ⇒  132-columns.
//	Ps = 2    ⇒  Printer.
//	Ps = 3    ⇒  ReGIS graphics.
//	Ps = 4    ⇒  Sixel graphics.
//	Ps = 6    ⇒  Selective erase.
//	Ps = 8    ⇒  User-defined keys.
//	Ps = 9    ⇒  National Replacement Character sets.
//	Ps = 1 5  ⇒  Technical characters.
//	Ps = 1 6  ⇒  Locator port.
//	Ps = 1 7  ⇒  Terminal state interrogation.
//	Ps = 1 8  ⇒  User windows.
//	Ps = 2 1  ⇒  Horizontal scrolling.
//	Ps = 2 2  ⇒  ANSI color, e.g., VT525.
//	Ps = 2 8  ⇒  Rectangular editing.
//	Ps = 2 9  ⇒  ANSI text locator (i.e., DEC Locator mode).
//
// NOTE: must be connected to an actual terminal for this to work
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h4-Functions-using-CSI-_-ordered-by-the-final-character-lparen-s-rparen:CSI-Ps-c.1CA3
func termAttributes(in, out *os.File) ([]int, error) {
	text, err := termRequestResponse(in, out, "\x1b[0c")
	if err != nil {
		return nil, err
	}
	m := numRE.FindAll(text, -1)
	attrs := make([]int, len(m))
	for i, b := range m {
		attrs[i], _ = strconv.Atoi(string(b))
	}
	return attrs, nil
}

var numRE = regexp.MustCompile(`\d+`)

// termRequestResponse handles request/response terminal control sequences like
// <ESC>[0c STDIN & STDOUT are parameterized for special cases. os.Stdin &
// os.Stdout are usually sufficient. `sRq` should be the request control
// sequence to the terminal. NOTE: only captures up to 1KB of response NOTE:
// when println debugging the response, probably want to go-escape it, like:
//
//	fmt.Printf("%#v\n", sRsp)
//
// since most responses begin with <ESC>, which the terminal treats as another
// control sequence rather than text to output.
func termRequestResponse(in, out *os.File, sRq string) (sRsp []byte, err error) {
	fd := int(in.Fd())
	// NOTE: raw mode tip came from https://play.golang.org/p/kcMLTiDRZY
	if !term.IsTerminal(fd) {
		return nil, ErrNonTTY
	}
	// stdin "raw mode" to capture terminal response
	// NOTE: without this, response bypasses stdin, and is written directly to
	// the console
	var old *term.State
	if old, err = term.MakeRaw(fd); err != nil {
		return
	}
	defer func() {
		// capture restore error (if any) if there hasn't already been an error
		if e := term.Restore(fd, old); err == nil {
			err = e
		}
	}()
	// send request
	if _, err = out.WriteString(sRq); err != nil {
		return
	}
	buf := make([]byte, 1024)
	// wait 1/16 second for term response.  if timer expires, trigger bytes to
	// stdin so .read() can finish
	t := time.NewTimer(time.Second >> 4)
	done := make(chan bool)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		select {
		case <-t.C:
			// "Report Cursor Position (CPR) [row; column]
			// just to get some bytes to stdin
			// NOTE: seems to work for everything except mlterm
			_, _ = out.WriteString("\x1b\x1b[" + "6n")
			break
		case <-done:
			break
		}
		wg.Done()
	}()
	// capture response
	n, err := in.Read(buf)
	// ensure termination
	if t.Stop() {
		done <- true
	} else {
		err = ErrTermResponseTimedOut
	}
	wg.Wait()
	if n > 0 && errors.Is(err, ErrTermResponseTimedOut) {
		return buf[:n], nil
	}
	return nil, err
}
