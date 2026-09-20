package sqlfuncs

import (
	"encoding/json"
	"errors"
	"strings"
)

// errInvalidJSONArgs mirrors the "Invalid JSON strings" error the retired C
// implementation raised for NULL arguments (json_equal.c).
var errInvalidJSONArgs = errors.New("Invalid JSON strings")

// errJSONParse mirrors the "Error parsing JSON strings" error the retired C
// implementation raised when either document failed to parse.
var errJSONParse = errors.New("Error parsing JSON strings")

// JSONEqual implements json_equal(a, b): 1 if the two JSON documents are
// deeply equal, else 0. Objects compare unordered with case-sensitive keys;
// arrays compare ordered; numbers compare as IEEE 754 doubles with the same
// epsilon rule as the retired cJSON implementation, so 1 and 1.0 are equal.
// NULL arguments and unparseable documents are errors, matching the C code.
func JSONEqual(a, b any) (any, error) {
	sa, okA := valueText(a)
	sb, okB := valueText(b)
	if !okA || !okB {
		return nil, errInvalidJSONArgs
	}
	va, errA := parseJSONDocument(sa)
	vb, errB := parseJSONDocument(sb)
	if errA != nil || errB != nil {
		return nil, errJSONParse
	}
	if jsonDeepEqual(va, vb) {
		return int64(1), nil
	}
	return int64(0), nil
}

// parseJSONDocument parses exactly one JSON value. Unlike the retired cJSON
// parser it rejects trailing non-whitespace content (see DIVERGENCES.md).
// Numbers decode to float64, the same binary64 representation cJSON used.
func parseJSONDocument(s string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("trailing content after JSON value")
	}
	return v, nil
}

// jsonDeepEqual ports cJSON_Compare(a, b, case_sensitive=true): unordered
// object comparison with case-sensitive keys, ordered array comparison and
// epsilon number comparison.
func jsonDeepEqual(a, b any) bool {
	switch av := a.(type) {
	case nil:
		return b == nil
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case float64:
		bv, ok := b.(float64)
		return ok && compareDouble(av, bv)
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !jsonDeepEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			bItem, present := bv[k]
			if !present || !jsonDeepEqual(v, bItem) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
