package envx_test

import (
	"sort"
	"testing"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/stretchr/testify/require"
)

func TestBuilder(t *testing.T) {
	t.Run("Setenv", func(t *testing.T) {
		t.Run("set_existing_variable", func(t *testing.T) {
			b := envx.Build().FromEnviron("KEY1=val1", "KEY2=val2")
			require.NoError(t, b.Setenv("KEY1", "new_val"))
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"KEY1=new_val", "KEY2=val2"}
			sort.Strings(actual)
			sort.Strings(expected)
			require.Equal(t, expected, actual)
		})

		t.Run("set_non_existing_variable", func(t *testing.T) {
			b := envx.Build().FromEnviron("KEY1=val1")
			require.NoError(t, b.Setenv("KEY2", "new_val"))
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"KEY1=val1"}
			require.Equal(t, expected, actual)
		})

		t.Run("set_variable_in_empty_builder", func(t *testing.T) {
			b := envx.Build()
			require.NoError(t, b.Setenv("KEY1", "new_val"))
			actual, err := b.Environ()
			require.NoError(t, err)
			require.Empty(t, actual)
		})

		t.Run("set_variable_with_multiple_occurrences", func(t *testing.T) {
			b := envx.Build().FromEnviron("KEY1=val1", "KEY2=val2", "KEY1=val1_again")
			require.NoError(t, b.Setenv("KEY1", "new_val"))
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"KEY1=new_val", "KEY1=new_val", "KEY2=val2"}
			sort.Strings(actual)
			sort.Strings(expected)
			require.Equal(t, expected, actual)
		})

		t.Run("set_variable_with_empty_value", func(t *testing.T) {
			b := envx.Build().FromEnviron("KEY1=val1")
			require.NoError(t, b.Setenv("KEY1", ""))
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"KEY1="}
			require.Equal(t, expected, actual)
		})
	})

	t.Run("Unsetenv", func(t *testing.T) {
		t.Run("unset_existing_variable", func(t *testing.T) {
			b := envx.Build().FromEnviron("KEY1=val1", "KEY2=val2")
			require.NoError(t, b.Unsetenv("KEY1"))
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"KEY2=val2"}
			require.Equal(t, expected, actual)
		})

		t.Run("unset_non_existing_variable", func(t *testing.T) {
			b := envx.Build().FromEnviron("KEY1=val1", "KEY2=val2")
			require.NoError(t, b.Unsetenv("KEY3"))
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"KEY1=val1", "KEY2=val2"}
			sort.Strings(actual)
			sort.Strings(expected)
			require.Equal(t, expected, actual)
		})

		t.Run("unset_from_empty_builder", func(t *testing.T) {
			b := envx.Build()
			require.NoError(t, b.Unsetenv("KEY1"))
			actual, err := b.Environ()
			require.NoError(t, err)
			require.Empty(t, actual)
		})

		t.Run("unset_variable_with_multiple_occurrences", func(t *testing.T) {
			b := envx.Build().FromEnviron("KEY1=val1", "KEY2=val2", "KEY1=val1_again")
			require.NoError(t, b.Unsetenv("KEY1"))
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"KEY2=val2"}
			require.Equal(t, expected, actual)
		})
	})

	t.Run("Drop", func(t *testing.T) {
		t.Run("drop_single_key", func(t *testing.T) {
			b := envx.Build().FromEnviron("A=1", "B=2", "C=3")
			b.Drop("B")
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"A=1", "C=3"}
			sort.Strings(actual)
			sort.Strings(expected)
			require.Equal(t, expected, actual)
		})

		t.Run("drop_multiple_keys", func(t *testing.T) {
			b := envx.Build().FromEnviron("A=1", "B=2", "C=3", "D=4")
			b.Drop("A", "C")
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"B=2", "D=4"}
			sort.Strings(actual)
			sort.Strings(expected)
			require.Equal(t, expected, actual)
		})

		t.Run("drop_non_existent_key", func(t *testing.T) {
			b := envx.Build().FromEnviron("A=1", "B=2")
			b.Drop("C")
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"A=1", "B=2"}
			sort.Strings(actual)
			sort.Strings(expected)
			require.Equal(t, expected, actual)
		})

		t.Run("drop_all_keys", func(t *testing.T) {
			b := envx.Build().FromEnviron("A=1", "B=2")
			b.Drop("A", "B")
			actual, err := b.Environ()
			require.NoError(t, err)
			require.Empty(t, actual)
		})

		t.Run("drop_from_empty_builder", func(t *testing.T) {
			b := envx.Build()
			b.Drop("A")
			actual, err := b.Environ()
			require.NoError(t, err)
			require.Empty(t, actual)
		})

		t.Run("drop_handles_duplicates", func(t *testing.T) {
			b := envx.Build().FromEnviron("A=1", "B=2", "A=3")
			b.Drop("A")
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"B=2"}
			require.Equal(t, expected, actual)
		})
	})

	t.Run("Only", func(t *testing.T) {
		t.Run("only_keeps_specified_keys", func(t *testing.T) {
			b := envx.Build().FromEnviron("A=1", "B=2", "C=3")
			b.Only("A", "C")
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"A=1", "C=3"}
			sort.Strings(actual)
			sort.Strings(expected)
			require.Equal(t, expected, actual)
		})

		t.Run("only_with_non_existent_key", func(t *testing.T) {
			b := envx.Build().FromEnviron("A=1", "B=2")
			b.Only("A", "C")
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"A=1"}
			require.Equal(t, expected, actual)
		})

		t.Run("only_with_empty_key_list", func(t *testing.T) {
			b := envx.Build().FromEnviron("A=1", "B=2")
			b.Only()
			actual, err := b.Environ()
			require.NoError(t, err)
			require.Empty(t, actual)
		})

		t.Run("only_on_empty_builder", func(t *testing.T) {
			b := envx.Build()
			b.Only("A")
			actual, err := b.Environ()
			require.NoError(t, err)
			require.Empty(t, actual)
		})

		t.Run("only_handles_duplicates", func(t *testing.T) {
			b := envx.Build().FromEnviron("A=1", "B=2", "A=3")
			b.Only("A")
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"A=3"}
			require.Equal(t, expected, actual)
		})

		t.Run("only_keeps_all_keys", func(t *testing.T) {
			b := envx.Build().FromEnviron("A=1", "B=2", "A=3")
			b.Only("A", "B")
			actual, err := b.Environ()
			require.NoError(t, err)
			expected := []string{"A=3", "B=2"}
			sort.Strings(actual)
			sort.Strings(expected)
			require.Equal(t, expected, actual)
		})
	})
}
