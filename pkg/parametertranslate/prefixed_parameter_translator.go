package parametertranslate

import (
	"fmt"
	"strings"

	"github.com/stackql/any-sdk/pkg/fuzzymatch"
)

func NewPrefixedTranslator(prefix string, attributeMatcher fuzzymatch.FuzzyMatcher[string]) ParameterTranslator {
	return &PrefixedTranslator{
		prefix:               prefix,
		allowedValuesMatcher: attributeMatcher,
	}
}

type PrefixedTranslator struct {
	prefix               string
	allowedValuesMatcher fuzzymatch.FuzzyMatcher[string]
}

func (gp *PrefixedTranslator) Translate(input string) (string, error) {
	return fmt.Sprintf("%s%s", gp.prefix, input), nil
}

func (gp *PrefixedTranslator) ReverseTranslate(input string) (string, error) {
	if len(input) < len(gp.prefix) {
		return "", fmt.Errorf("PrefixedTranslator.ReverseTRanslate(): prefixed trinput string is too short to remove prefix")
	}
	trimmed := strings.TrimPrefix(input, gp.prefix)
	if trimmed == input {
		return "", fmt.Errorf("PrefixedTranslator.ReverseTRanslate(): input string does not contain prefix")
	}
	return trimmed, nil
}
