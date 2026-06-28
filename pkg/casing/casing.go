// Package casing provides casing transforms between a snake_case SQL surface and
// the "native" wire casing used by a foreign API (pascal | kebab | camel | snake).
//
// The forward (snake) transform is a port of botocore's xform_name, so that the
// snake aliases stackql exposes match what the AWS CLI produces for top-level
// argument names. The transform is applied to TOP-LEVEL identifiers only; nested
// struct contents are passed verbatim by the caller and are not transformed here.
package casing

import (
	"regexp"
	"strings"
	"sync"
)

// Native casing identifiers, as carried by a method's request.nativeCasing.
const (
	Snake  = "snake"
	Pascal = "pascal"
	Kebab  = "kebab"
	Camel  = "camel"
)

// botocore xform_name regexes (verbatim ports).
var (
	firstCapRe  = regexp.MustCompile(`(.)([A-Z][a-z]+)`)
	numberCapRe = regexp.MustCompile(`([a-z])([0-9]+)`)
	endCapRe    = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	specialRe   = regexp.MustCompile(`[A-Z]{2,}s$`)
)

// snakeCache memoises ToSnake, mirroring botocore's _xform_cache.
var snakeCache sync.Map // map[string]string

// ToSnake converts a wire identifier (PascalCase / camelCase) to snake_case using
// botocore's xform_name algorithm with '_' as the separator. Acronyms collapse
// (VPCId -> vpc_id, VPCEndpoint -> vpc_endpoint); the transform is intentionally
// lossy for acronyms, exactly as the AWS CLI is.
func ToSnake(name string) string {
	if v, ok := snakeCache.Load(name); ok {
		return v.(string)
	}
	out := xform(name, "_")
	snakeCache.Store(name, out)
	return out
}

func xform(name, sep string) string {
	// If the separator is already present, botocore treats the name as final.
	if strings.Contains(name, sep) {
		return name
	}
	if matched := specialRe.FindString(name); matched != "" {
		// e.g. "ARNs" -> "ar" + sep + "ns" before the generic passes.
		name = name[:len(name)-len(matched)] + sep + strings.ToLower(matched)
	}
	s1 := firstCapRe.ReplaceAllString(name, "${1}"+sep+"${2}")
	s2 := numberCapRe.ReplaceAllString(s1, "${1}"+sep+"${2}")
	s3 := endCapRe.ReplaceAllString(s2, "${1}"+sep+"${2}")
	return strings.ToLower(s3)
}

// FromSnake converts a snake_case identifier to the given native casing. It is the
// inverse used by reverse-casing parameter lookup: a snake SQL key is converted to
// the wire casing and re-resolved against the parameter / property set.
func FromSnake(snake, nativeCasing string) string {
	switch nativeCasing {
	case Pascal:
		return ToPascal(snake)
	case Camel:
		return ToCamel(snake)
	case Kebab:
		return ToKebab(snake)
	case Snake, "":
		return snake
	default:
		return snake
	}
}

// ToPascal converts snake_case to PascalCase (vpc_id -> VpcId).
func ToPascal(snake string) string {
	return joinCaps(strings.Split(snake, "_"), true)
}

// ToCamel converts snake_case to camelCase (vpc_id -> vpcId).
func ToCamel(snake string) string {
	return joinCaps(strings.Split(snake, "_"), false)
}

// ToKebab converts snake_case to kebab-case (vpc_id -> vpc-id).
func ToKebab(snake string) string {
	return strings.ReplaceAll(snake, "_", "-")
}

// joinCaps capitalises the first letter of each segment; when capitaliseFirst is
// false the first segment is left lower-cased (camelCase).
func joinCaps(segments []string, capitaliseFirst bool) string {
	var b strings.Builder
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		if i == 0 && !capitaliseFirst {
			b.WriteString(seg)
			continue
		}
		b.WriteString(strings.ToUpper(seg[:1]))
		b.WriteString(seg[1:])
	}
	return b.String()
}

// IsKnownCasing reports whether s is a recognised native casing identifier.
func IsKnownCasing(s string) bool {
	switch s {
	case Snake, Pascal, Kebab, Camel:
		return true
	default:
		return false
	}
}
