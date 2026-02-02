package envx_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestExpandInPlace(t *testing.T) {
	t.Run("example 1 - no substitutions", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "derp")
		require.NoError(t, os.WriteFile(path, []byte("DERP DERP DERP\n"), 0600))
		require.Equal(t, "9b00bbb6-309c-37b8-befe-0683616abbb3", testx.ReadMD5(path))
		require.NoError(t, envx.ExpandInplace(path, func(s string) string { return "" }))
		require.Equal(t, "DERP DERP DERP\n", testx.ReadString(path))
		require.Equal(t, "9b00bbb6-309c-37b8-befe-0683616abbb3", testx.ReadMD5(path))
	})

	t.Run("example 1 - no substitutions", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "derp")
		require.NoError(t, os.WriteFile(path, []byte("hello ${FOO}\n"), 0600))
		require.Equal(t, "efb23271-f481-f104-244d-a02786427080", testx.ReadMD5(path))
		require.NoError(t, envx.ExpandInplace(path, func(s string) string {
			switch s {
			case "FOO":
				return "BAR"
			default:
				return fmt.Sprintf("${%s}", s)
			}
		}))
		require.Equal(t, "hello BAR\n", testx.ReadString(path))
		require.Equal(t, "6551a0cd-09f3-9c1a-837a-62fa607c405c", testx.ReadMD5(path))
	})
}

func TestExpandReader(t *testing.T) {
	standardMapping := func(key string) string {
		switch key {
		case "NAME":
			return "Alice"
		case "COLOR":
			return "Blue"
		case "GREETING":
			return "Hello"
		case "LOCATION":
			return "Boston"
		default:
			return "" // Default behavior for unknown variables: replace with empty string
		}
	}

	// --- Simple Expansion Scenarios ---
	t.Run("simple_expansion", func(t *testing.T) {
		t.Run("all_variables_found", func(t *testing.T) {
			input := `
$GREETING, $NAME!
Your favorite color is ${COLOR} in ${LOCATION}.
`
			expected := `
Hello, Alice!
Your favorite color is Blue in Boston.
`
			reader := strings.NewReader(input)
			expandedReader := envx.ExpandReader(reader, standardMapping)
			output := testx.MustT(io.ReadAll(expandedReader))(t)
			require.EqualValues(t, expected, output, "Expanded output should match expected for all variables found")
		})

		t.Run("some_variables_not_found", func(t *testing.T) {
			input := `
$GREETING, $NAME!
Unknown var: $UNKNOWN.
Another unknown: ${ANOTHER}.
`
			expected := `
Hello, Alice!
Unknown var: .
Another unknown: .
` // Assuming empty string for unknown vars by default mapping
			reader := strings.NewReader(input)
			expandedReader := envx.ExpandReader(reader, standardMapping)
			output := testx.MustT(io.ReadAll(expandedReader))(t)
			require.EqualValues(t, expected, output, "Expanded output should replace unknown variables with empty string")
		})

		t.Run("empty_mapping", func(t *testing.T) {
			input := "$GREETING, $NAME!"
			expected := ", !\n"
			reader := strings.NewReader(input)
			expandedReader := envx.ExpandReader(reader, func(string) string { return "" })
			output := testx.MustT(io.ReadAll(expandedReader))(t)
			require.EqualValues(t, expected, string(output), "Expanded output should be correct with an empty mapping")
		})
	})

	// --- Edge Cases ---
	t.Run("edge_cases", func(t *testing.T) {
		t.Run("empty_input_reader", func(t *testing.T) {
			reader := strings.NewReader("")
			expandedReader := envx.ExpandReader(reader, standardMapping)
			output := testx.MustT(io.ReadAll(expandedReader))(t)
			require.Empty(t, output, "Empty input should result in empty output")
		})

		t.Run("input_with_no_variables", func(t *testing.T) {
			input := `
This is a plain line.
Another line without variables.
`
			reader := strings.NewReader(input)
			expandedReader := envx.ExpandReader(reader, standardMapping)
			output := testx.MustT(io.ReadAll(expandedReader))(t)
			require.EqualValues(t, input, output, "Input without variables should be returned unchanged")
		})

		t.Run("input_ends_without_newline", func(t *testing.T) {
			input := "Last line no newline $VAR"
			// Our envx.ExpandReader implementation adds a newline because bufio.Scanner.Text() strips it.
			expected := "Last line no newline ExpandedVar\n"
			mapping := func(key string) string {
				if key == "VAR" {
					return "ExpandedVar"
				}
				return ""
			}
			reader := strings.NewReader(input)
			expandedReader := envx.ExpandReader(reader, mapping)
			output := testx.MustT(io.ReadAll(expandedReader))(t)
			require.EqualValues(t, expected, output, "Last line without newline should have one added after expansion")
		})

		t.Run("input_with_only_newlines", func(t *testing.T) {
			input := "\n\n\n"
			expected := "\n\n\n"
			reader := strings.NewReader(input)
			expandedReader := envx.ExpandReader(reader, standardMapping)
			output := testx.MustT(io.ReadAll(expandedReader))(t)
			require.EqualValues(t, expected, output, "Input with only newlines should yield identical output")
		})
	})

	// --- Buffer Size Handling (Crucial for io.Reader implementation) ---
	t.Run("buffer_size_handling", func(t *testing.T) {
		t.Run("large_expanded_line_fits_small_buffer", func(t *testing.T) {
			longValue := strings.Repeat("X", 100) // An expanded value of 100 characters
			input := "Line with $LONG_VAR_HERE.\nAnother line."
			// Expected output includes the newline added by envx.ExpandReader
			expected := fmt.Sprintf("Line with %s.\nAnother line.\n", longValue)

			mapping := func(key string) string {
				if key == "LONG_VAR_HERE" {
					return longValue
				}
				return ""
			}

			reader := strings.NewReader(input)
			expandedReader := envx.ExpandReader(reader, mapping)

			var outputBuffer bytes.Buffer
			buf := make([]byte, 10) // A very small buffer (10 bytes) for Read calls
			totalRead := 0
			for {
				n, err := expandedReader.Read(buf)
				if n > 0 {
					outputBuffer.Write(buf[:n])
					totalRead += n
				}
				if err == io.EOF {
					break
				}
				require.NoError(t, err, "Expected no error during read until EOF") // Any other error is unexpected
			}
			require.EqualValues(t, expected, outputBuffer.String(), "Output should be correct even with small read buffer")
			require.EqualValues(t, len(expected), totalRead, "Total bytes read should match expected length")
		})

		t.Run("multiple_calls_for_single_line_expansion", func(t *testing.T) {
			longValue := strings.Repeat("A", 50)                     // A value that will span multiple 10-byte Read calls
			input := "PREFIX-$VAR-SUFFIX\n"                          // Input line
			expected := fmt.Sprintf("PREFIX-%s-SUFFIX\n", longValue) // Expected expanded line

			mapping := func(key string) string {
				if key == "VAR" {
					return longValue
				}
				return ""
			}

			reader := strings.NewReader(input)
			expandedReader := envx.ExpandReader(reader, mapping)

			var outputBuffer bytes.Buffer
			buf := make([]byte, 10) // Small buffer
			for {
				n, err := expandedReader.Read(buf)
				if n > 0 {
					outputBuffer.Write(buf[:n])
				}
				if err == io.EOF {
					break
				}
				require.NoError(t, err, "Expected no error until EOF")
			}
			require.EqualValues(t, expected, outputBuffer.String(), "Single long line should be correctly read across multiple Read calls")
		})
	})

	t.Run("error_handling", func(t *testing.T) {
		t.Run("underlying_reader_returns_error", func(t *testing.T) {
			expandedReader := envx.ExpandReader(
				io.MultiReader(strings.NewReader("line1\nline2\nline3\n"), errorsx.Reader(fmt.Errorf("simulated underlying read error"))),
				standardMapping,
			)

			_, err := io.ReadAll(expandedReader)
			require.Error(t, err, "Expected an error from the expanded reader")
		})

		t.Run("read_after_EOF", func(t *testing.T) {
			reader := strings.NewReader("single line\n")
			expandedReader := envx.ExpandReader(reader, standardMapping)

			// Read all content to reach EOF
			_, err := io.ReadAll(expandedReader)
			require.NoError(t, err, "Should read successfully until EOF the first time")

			// Try to read again after EOF is reached.
			// Subsequent reads should consistently return 0, io.EOF.
			buf := make([]byte, 1)
			n, err := expandedReader.Read(buf)
			require.EqualValues(t, 0, n, "Should read 0 bytes after EOF")
			require.EqualValues(t, io.EOF, err, "Should return io.EOF after reaching end of stream")

			// Repeated read after EOF to ensure consistent behavior
			n, err = expandedReader.Read(buf)
			require.EqualValues(t, 0, n, "Should read 0 bytes on repeated call after EOF")
			require.EqualValues(t, io.EOF, err, "Should return io.EOF on repeated call after EOF")
		})
	})
}
