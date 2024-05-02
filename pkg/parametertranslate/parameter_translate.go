package parametertranslate

import (
	"strings"

	"github.com/stackql/any-sdk/pkg/fuzzymatch"
)

const (
	legacyPrefixedAttributes string = "prefixed_with_"
	naivePropertyAttributes  string = "naive_property_attributes"
)

type ParameterTranslator interface {
	Translate(string) (string, error)
	ReverseTranslate(string) (string, error)
}

func GetPrefixPrefix() string {
	return legacyPrefixedAttributes
}

func NewParameterTranslator(algorithm string, attributeMatcher fuzzymatch.FuzzyMatcher[string]) ParameterTranslator {
	switch strings.ToLower(algorithm) {
	case naivePropertyAttributes:
		return NewNilTranslator()
	default:
		if strings.HasPrefix(algorithm, legacyPrefixedAttributes) {
			return NewPrefixedTranslator(
				strings.TrimPrefix(algorithm, legacyPrefixedAttributes),
				attributeMatcher,
			)
		}
		return NewNilTranslator()
	}
}
