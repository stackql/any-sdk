package parametertranslate

import (
	"fmt"

	"github.com/stackql/any-sdk/pkg/fuzzymatch"
)

func NewNaiveBodyTranslator(prefix string, attributeMatcher fuzzymatch.FuzzyMatcher[string]) ParameterTranslator {
	return &NaiveBodyTranslator{
		prefix:               prefix,
		allowedValuesMatcher: attributeMatcher,
	}
}

type NaiveBodyTranslator struct {
	prefix               string
	allowedValuesMatcher fuzzymatch.FuzzyMatcher[string]
}

func (gp *NaiveBodyTranslator) Translate(input string) (string, error) {
	// return fmt.Sprintf("%s%s", gp.prefix, input), nil
	return input, nil
}

func (gp *NaiveBodyTranslator) ReverseTranslate(input string) (string, error) {
	rv, exists := gp.allowedValuesMatcher.Find(input)
	if exists {
		return rv, nil
	}
	return "", fmt.Errorf("NaiveBodyTranslator.ReverseTranslate(): input string does conform to allowed values")
}
