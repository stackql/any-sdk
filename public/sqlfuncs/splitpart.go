package sqlfuncs

import "strings"

// SplitPart implements split_part(source, separator, part): it splits source
// on separator and returns the part selected by the one-based index part.
// Negative indexes count backward from the end (-1 is the last part).
// Behavior ported from the retired C implementation (split_part.c):
//   - NULL source, NULL separator or empty separator yields NULL.
//   - A NULL part index coerces to 0, which is out of range, yielding NULL.
//   - Consecutive separators produce empty parts.
//   - An out-of-range index yields NULL.
func SplitPart(source, separator, part any) (any, error) {
	src, srcOK := valueText(source)
	sep, sepOK := valueText(separator)
	if !srcOK || !sepOK || sep == "" {
		return nil, nil
	}
	idx := valueInt(part)
	parts := strings.Split(src, sep)
	n := int64(len(parts))
	if idx < 0 {
		idx = n + idx
	} else {
		idx--
	}
	if idx >= 0 && idx < n {
		return parts[idx], nil
	}
	return nil, nil
}
