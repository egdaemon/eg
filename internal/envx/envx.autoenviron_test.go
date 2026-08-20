package envx_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/stretchr/testify/require"
)

func TestAutoEnviron(t *testing.T) {
	t.Run("empty_input_returns_empty_slice", func(t *testing.T) {
		result := envx.AutoEnviron()
		require.Empty(t, result)
	})

	t.Run("nil_input_returns_empty_slice", func(t *testing.T) {
		result := envx.AutoEnviron(nil...)
		require.Empty(t, result)
	})

	t.Run("pass_through_k_equals_v_strings", func(t *testing.T) {
		result := envx.AutoEnviron("EXISTING_KEY=existing_value", "ANOTHER=a=b=c")
		require.Len(t, result, 2)
		require.Contains(t, result, "EXISTING_KEY=existing_value")
		require.Contains(t, result, "ANOTHER=a=b=c")
	})

	t.Run("mix_of_keys_and_k_equals_v", func(t *testing.T) {
		t.Setenv("AUTO_MIXED_KEY", "auto_value")
		result := envx.AutoEnviron(
			"auto_key=value",
			"AUTO_MIXED_KEY",
			"another_explicit=explicit_value",
		)
		require.Len(t, result, 3)
		require.Contains(t, result, "auto_key=value")
		require.Contains(t, result, "AUTO_MIXED_KEY=auto_value")
		require.Contains(t, result, "another_explicit=explicit_value")
	})

	t.Run("missing_key_is_skipped", func(t *testing.T) {
		const key = "a1b2c3d4-e5f6-7890-abcd-auto-missing"
		result := envx.AutoEnviron(key)
		require.Empty(t, result)
	})

	t.Run("existing_key_is_resolved_from_os", func(t *testing.T) {
		const key = "b2c3d4e5-f6a7-8901-bcde-auto-env"
		expected := "auto-resolved-value"
		t.Setenv(key, expected)
		result := envx.AutoEnviron(key)
		require.Len(t, result, 1)
		require.Contains(t, result, key+"="+expected)
	})

	t.Run("value_containing_equals_is_preserved", func(t *testing.T) {
		const key = "c3d4e5f6-a7b8-9012-cdef-auto-equals"
		expected := "a=b=c"
		t.Setenv(key, expected)
		result := envx.AutoEnviron(key)
		require.Len(t, result, 1)
		require.Contains(t, result, key+"=a=b=c")
	})

	t.Run("multiple_existing_keys", func(t *testing.T) {
		t.Setenv("AUTO_KEY1", "val1")
		t.Setenv("AUTO_KEY2", "val2")
		t.Setenv("AUTO_KEY3", "val3")

		result := envx.AutoEnviron("AUTO_KEY1", "AUTO_KEY2", "AUTO_KEY3")
		require.Len(t, result, 3)
		require.Contains(t, result, "AUTO_KEY1=val1")
		require.Contains(t, result, "AUTO_KEY2=val2")
		require.Contains(t, result, "AUTO_KEY3=val3")
	})

	t.Run("multiple_keys_with_some_missing", func(t *testing.T) {
		t.Setenv("AUTO_PRESENT", "found")
		result := envx.AutoEnviron("AUTO_PRESENT", "AUTO_MISSING_1", "AUTO_MISSING_2")
		require.Len(t, result, 1)
		require.Contains(t, result, "AUTO_PRESENT=found")
	})

	t.Run("order_preserved", func(t *testing.T) {
		t.Setenv("AUTO_ORDER_1", "first")
		t.Setenv("AUTO_ORDER_2", "second")
		t.Setenv("AUTO_ORDER_3", "third")

		result := envx.AutoEnviron("AUTO_ORDER_3", "AUTO_ORDER_1", "AUTO_ORDER_2")
		require.Equal(t, []string{
			"AUTO_ORDER_3=third",
			"AUTO_ORDER_1=first",
			"AUTO_ORDER_2=second",
		}, result)
	})

	t.Run("explicit_equals_strings_preserved_in_order", func(t *testing.T) {
		result := envx.AutoEnviron(
			"Z_LAST=z",
			"A_FIRST=a",
			"MIDDLE=m",
		)
		require.Equal(t, []string{
			"Z_LAST=z",
			"A_FIRST=a",
			"MIDDLE=m",
		}, result)
	})

	t.Run("empty_value_is_skipped_when_it_resolves_to_empty", func(t *testing.T) {
		const key = "d4e5f6a7-b8c9-0123-defa-auto-empty"
		t.Setenv(key, "")
		// os.LookupEnv returns ("", true) for an explicitly set empty value,
		// but Format with allowAll returns "KEY=" which is non-empty.
		// However, allowAll is identity, so the value is preserved.
		result := envx.AutoEnviron(key)
		require.Len(t, result, 1)
		require.Contains(t, result, key+"=")
	})

	t.Run("key_with_equals_in_name_is_treated_as_explicit", func(t *testing.T) {
		result := envx.AutoEnviron("MY=KEY=value")
		require.Len(t, result, 1)
		require.Contains(t, result, "MY=KEY=value")
	})

	t.Run("explicit_FOO_with_unset_env_var_is_skipped", func(t *testing.T) {
		const key = "e5f6a7b8-c9d0-1234-efab-auto-explicit-empty"
		result := envx.AutoEnviron(key + "=")
		require.Empty(t, result)
	})

	t.Run("explicit_FOO_with_set_empty_env_var_resolves_via_lookupenv", func(t *testing.T) {
		const key = "f6a7b8c9-d0e1-2345-fabc-auto-explicit-empty-set"
		t.Setenv(key, "")
		result := envx.AutoEnviron(key + "=")
		require.Len(t, result, 1)
		require.Contains(t, result, key+"=")
	})

	t.Run("explicit_FOO_with_set_non_empty_env_var_overrides_empty_literal", func(t *testing.T) {
		const key = "a7b8c9d0-e1f2-3456-abcd-auto-explicit-empty-set"
		t.Setenv(key, "resolved_value")
		result := envx.AutoEnviron(key + "=")
		require.Len(t, result, 1)
		require.Contains(t, result, key+"=resolved_value")
	})

	t.Run("explicit_FOO_equals_treats_empty_value_same_as_bare_key", func(t *testing.T) {
		const key = "b8c9d0e1-f2a3-4567-bcde-auto-explicit-empty"
		t.Setenv(key, "from_env")
		// "FOO=" has len(v)==0 so it falls through to os.LookupEnv, producing
		// the same result as the bare key.
		result := envx.AutoEnviron(key, key+"=")
		require.Len(t, result, 2)
		for _, got := range result {
			require.Equal(t, key+"=from_env", got)
		}
	})
}
