package stream_transform

import (
	"fmt"
	"regexp"
)

type RegexpShorthand interface {
	GetFirstMatch(string, string) (string, error)
	GetAllMatches(string, string) ([]string, error)
}

type regexpShorthand struct {
}

func (rs *regexpShorthand) GetFirstMatch(input string, pattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	match := re.FindStringSubmatch(input)
	if len(match) < 2 {
		return "", fmt.Errorf("no match found for pattern %q in input %q", pattern, input)
	}
	return match[1], nil
}

func (rs *regexpShorthand) GetAllMatches(input string, pattern string) ([]string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	match := re.FindStringSubmatch(input)
	if len(match) < 2 {
		return nil, fmt.Errorf("no match found for pattern %q in input %q", pattern, input)
	}
	return match[1:], nil
}

func NewRegexpShorthand() RegexpShorthand {
	return &regexpShorthand{}
}
