package sqlfuncs

import (
	"errors"
	"sort"
	"strings"
)

// errInvalidPolicyArgs mirrors the "Invalid policy strings" error the retired
// C implementation raised for NULL arguments (aws_policy_equal.c).
var errInvalidPolicyArgs = errors.New("Invalid policy strings") //nolint:staticcheck // C-parity error text

// errPolicyParse mirrors the "Error parsing policy JSON strings" error the
// retired C implementation raised when either document failed to parse.
var errPolicyParse = errors.New("Error parsing policy JSON strings") //nolint:staticcheck // C-parity error text

// awsUnorderedFields are the policy fields whose array values compare as
// unordered sets and whose ARN string values compare case-insensitively,
// exactly the list in the retired C implementation.
var awsUnorderedFields = map[string]struct{}{
	"Action":       {},
	"NotAction":    {},
	"Resource":     {},
	"NotResource":  {},
	"Principal":    {},
	"NotPrincipal": {},
	"AWS":          {},
	"Service":      {},
}

// AWSPolicyEqual implements aws_policy_equal(a, b): 1 if the two AWS IAM
// policy documents are semantically equivalent per the retired C
// implementation's normalization rules, else 0. Those rules are:
//   - identical strings are equal without parsing;
//   - a single-element array is equal to its lone string element in any
//     position (AWS scalar-vs-array forms);
//   - arrays under Action / NotAction / Resource / NotResource / Principal /
//     NotPrincipal / AWS / Service compare as unordered sets, all other
//     arrays (including Statement) compare ordered;
//   - within those unordered contexts, strings starting with "arn:" compare
//     ASCII case-insensitively;
//   - object keys compare unordered and are looked up case-insensitively.
//
// NULL arguments and unparseable documents are errors, matching the C code.
func AWSPolicyEqual(a, b any) (any, error) {
	sa, okA := valueText(a)
	sb, okB := valueText(b)
	if !okA || !okB {
		return nil, errInvalidPolicyArgs
	}
	if sa == sb {
		return int64(1), nil
	}
	va, errA := parseJSONDocument(sa)
	vb, errB := parseJSONDocument(sb)
	if errA != nil || errB != nil {
		return nil, errPolicyParse
	}
	if awsPolicyCompare(va, vb, false) {
		return int64(1), nil
	}
	return int64(0), nil
}

// awsPolicyCompare ports aws_policy_compare_items from the retired C
// implementation, including its deliberate asymmetries (the "arn:" prefix is
// tested on the left operand only, and unordered array elements are matched
// left-into-right after a length check).
//
//nolint:gocognit // faithful port of the C comparison, kept in one unit
func awsPolicyCompare(a, b any, parentUnordered bool) bool {
	// Special case: a single-element array compares equal to a lone string.
	if aArr, ok := a.([]any); ok {
		if bStr, isStr := b.(string); isStr {
			return len(aArr) == 1 && awsPolicyCompare(aArr[0], bStr, parentUnordered)
		}
	}
	if bArr, ok := b.([]any); ok {
		if aStr, isStr := a.(string); isStr {
			return len(bArr) == 1 && awsPolicyCompare(bArr[0], aStr, parentUnordered)
		}
	}
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
		if !ok {
			return false
		}
		if parentUnordered && strings.HasPrefix(av, "arn:") {
			return asciiEqualFold(av, bv)
		}
		return av == bv
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		if parentUnordered {
			for _, aElem := range av {
				if !awsFindMatchingElement(bv, aElem) {
					return false
				}
			}
			return true
		}
		for i := range av {
			if !awsPolicyCompare(av[i], bv[i], parentUnordered) {
				return false
			}
		}
		return true
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			return false
		}
		for k, v := range av {
			_, fieldUnordered := awsUnorderedFields[k]
			bItem, present := awsObjectGet(bv, k)
			if !present || !awsPolicyCompare(v, bItem, fieldUnordered) {
				return false
			}
		}
		for k := range bv {
			if _, present := awsObjectGet(av, k); !present {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// awsFindMatchingElement reports whether item matches any element of array
// under unordered comparison, porting find_matching_element from the C code.
func awsFindMatchingElement(array []any, item any) bool {
	for _, elem := range array {
		if awsPolicyCompare(elem, item, true) {
			return true
		}
	}
	return false
}

// awsObjectGet ports cJSON_GetObjectItem's case-insensitive key lookup. An
// exact match wins; otherwise the lexicographically smallest key that folds
// equal is used so the result is deterministic (cJSON used insertion order,
// which Go maps do not preserve).
func awsObjectGet(obj map[string]any, key string) (any, bool) {
	if v, ok := obj[key]; ok {
		return v, true
	}
	var candidates []string
	for k := range obj {
		if asciiEqualFold(k, key) {
			candidates = append(candidates, k)
		}
	}
	if len(candidates) == 0 {
		return nil, false
	}
	sort.Strings(candidates)
	return obj[candidates[0]], true
}
