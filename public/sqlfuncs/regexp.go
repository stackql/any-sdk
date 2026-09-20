package sqlfuncs

import (
	"fmt"
	"regexp"
)

// compilePattern compiles pattern with Go's regexp (RE2). The retired C
// engine (tiny-regex-c, built with RE_DOT_MATCHES_NEWLINE=1) matched any
// byte with '.', so patterns are compiled in single-line mode to preserve
// dot-matches-newline behavior. All other engine differences are documented
// in DIVERGENCES.md.
func compilePattern(pattern string) (*regexp.Regexp, error) {
	re, err := regexp.Compile("(?s)" + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression: %w", err)
	}
	return re, nil
}

// RegexpLike implements regexp_like(source, pattern): 1 if source contains a
// match for pattern, else 0. NULL source or pattern yields NULL. An invalid
// pattern is an error, mirroring the retired C implementation (regexp.c).
func RegexpLike(source, pattern any) (any, error) {
	src, srcOK := valueText(source)
	pat, patOK := valueText(pattern)
	if !srcOK || !patOK {
		return nil, nil
	}
	re, err := compilePattern(pat)
	if err != nil {
		return nil, err
	}
	if re.MatchString(src) {
		return int64(1), nil
	}
	return int64(0), nil
}

// RegexpSubstr implements regexp_substr(source, pattern): the first (leftmost)
// substring of source matching pattern, or NULL if there is no match. NULL
// source or pattern yields NULL. An empty pattern matches the empty string.
func RegexpSubstr(source, pattern any) (any, error) {
	src, srcOK := valueText(source)
	pat, patOK := valueText(pattern)
	if !srcOK || !patOK {
		return nil, nil
	}
	re, err := compilePattern(pat)
	if err != nil {
		return nil, err
	}
	loc := re.FindStringIndex(src)
	if loc == nil {
		return nil, nil
	}
	return src[loc[0]:loc[1]], nil
}

// RegexpReplace implements regexp_replace(source, pattern, replacement):
// every match of pattern in source is replaced with replacement. NULL in any
// argument yields NULL. The replacement text is literal - capture-group
// tokens such as $1 are not expanded - exactly like the retired C
// implementation, which copied the replacement verbatim.
func RegexpReplace(source, pattern, replacement any) (any, error) {
	src, srcOK := valueText(source)
	pat, patOK := valueText(pattern)
	rep, repOK := valueText(replacement)
	if !srcOK || !patOK || !repOK {
		return nil, nil
	}
	re, err := compilePattern(pat)
	if err != nil {
		return nil, err
	}
	return re.ReplaceAllLiteralString(src, rep), nil
}
