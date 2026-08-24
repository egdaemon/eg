package genieql

import (
	"strings"
	"unicode"

	"github.com/james-lawrence/genieql/internal/transformx"
	"github.com/serenize/snaker"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
)

// AliaserBuilder looks up transformations by name, if any of transformations
// do not exist returns nil.
func AliaserBuilder(names ...string) transform.Transformer {
	aliaserSet := make([]transform.Transformer, 0, len(names))
	for _, name := range names {
		aliaser := AliaserSelect(name)
		if aliaser == nil {
			return nil
		}
		aliaserSet = append(aliaserSet, aliaser)
	}

	return transform.Chain(aliaserSet...)
}

// AliaserSelect predefines common transformations for Aliases
func AliaserSelect(aliasername string) transform.Transformer {
	switch strings.ToLower(aliasername) {
	case "lowercase":
		return AliasStrategyLowercase
	case "uppercase":
		return AliasStrategyUppercase
	case "snakecase":
		return AliasStrategySnakecase
	case "camelcase":
		return AliasStrategyCamelcase
	default:
		return nil
	}
}

// AliasStrategyLowercase strategy for lowercasing field names to match result fields.
var AliasStrategyLowercase transform.Transformer = runes.Map(unicode.ToLower)

// AliasStrategyUppercase strategy for uppercasing field names to match result fields.
var AliasStrategyUppercase transform.Transformer = runes.Map(unicode.ToUpper)

// AliasStrategySnakecase strategy for snake casing field names to match result fields.
var AliasStrategySnakecase transform.Transformer = transformx.Full(snaker.CamelToSnake)

// AliasStrategyCamelcase strategy for camel casing field names to match result fields.
var AliasStrategyCamelcase transform.Transformer = transformx.Full(snaker.SnakeToCamel)
