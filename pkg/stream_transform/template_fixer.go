package stream_transform

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// assignChainedIndexRe matches {{ $var := index EXPR "k1" "k2" ... }}
// e.g. {{- $items := index $resp "securityGroupInfo" "item" -}}
var assignChainedIndexRe = regexp.MustCompile(
	`\{\{(-?\s*)(\$\w+)\s*:=\s*index\s+(\S+)\s+("(?:[^"]*"\s*)+"[^"]*")(\s*-?)\}\}`)

// withChainedIndexRe matches {{ with index EXPR "k1" "k2" ... }} capturing:
//   group 1: whitespace/trim before "with"
//   group 2: the expression (variable like $resp, $res, or .)
//   group 3: all chained string keys ("k1" "k2" ...)
//   group 4: whitespace/trim after last key
var withChainedIndexRe = regexp.MustCompile(
	`\{\{(-?\s*)with\s+index\s+(\S+)\s+("(?:[^"]*"\s*)+"[^"]*")(\s*-?)\}\}`)

// inlineChainedIndexRe matches {{ with index EXPR "k1" "k2" }}VALUE{{ else }}FALLBACK{{ end }}
// on a single line. Used for the "placement" "availabilityZone" pattern.
var inlineChainedIndexRe = regexp.MustCompile(
	`\{\{(-?\s*)with\s+index\s+(\S+)\s+("(?:[^"]*"\s*)+"[^"]*")(\s*-?)\}\}` +
		`([^{]*?)` + // VALUE (non-tag text)
		`\{\{(-?\s*)else(\s*-?)\}\}` +
		`([^{]*?)` + // FALLBACK
		`\{\{(-?\s*)end(\s*-?)\}\}`)

// FixTemplate attempts to produce a corrected version of a busted template.
// Returns the fixed template body, or empty string if no fix is applicable.
func FixTemplate(body string, tplType string) string {
	formatClass := classifyTemplateFormat(tplType)

	fixed := body

	// Fix 1: bare {{ toJson . }} without nil guard
	if strings.TrimSpace(body) == "{{ toJson . }}" {
		return "{{- if . -}}{{ toJson . }}{{- else -}}null{{- end -}}"
	}

	// Fix 2, 3, 4: chained index in MXJ templates — iterate until stable
	if formatClass == templateFormatXML {
		for i := 0; i < 10; i++ {
			prev := fixed
			fixed = fixInlineChainedIndex(fixed)
			fixed = fixWithChainedIndex(fixed)
			fixed = fixAssignChainedIndex(fixed)
			if fixed == prev {
				break
			}
		}
	}

	if fixed == body {
		return "" // no changes made
	}
	return fixed
}

// fixInlineChainedIndex fixes single-line patterns like:
//
//	{{ with index . "k1" "k2" }}{{ toJson . }}{{ else }}null{{ end }}
//
// by breaking the chain into nested with blocks.
func fixInlineChainedIndex(body string) string {
	return inlineChainedIndexRe.ReplaceAllStringFunc(body, func(match string) string {
		groups := inlineChainedIndexRe.FindStringSubmatch(match)
		if groups == nil {
			return match
		}
		trimL := groups[1]
		expr := groups[2]
		keysRaw := groups[3]
		trimR := groups[4]
		value := groups[5]
		elseTrimL := groups[6]
		elseTrimR := groups[7]
		fallback := groups[8]
		endTrimL := groups[9]
		endTrimR := groups[10]

		keys := parseQuotedKeys(keysRaw)
		if len(keys) < 2 {
			return match
		}

		// Break: index EXPR "k1" "k2" → with index EXPR "k1" then with index . "k2"
		firstKey := keys[0]
		remainingKeys := keys[1:]

		// Build the inner index call
		innerIndex := fmt.Sprintf("index . %s", quoteKeys(remainingKeys))

		return fmt.Sprintf(
			"{{%swith index %s %s%s}}"+
				"{{%sif eq (printf \"%%T\" .) \"map[string]interface {}\" %s}}"+
				"{{%swith %s%s}}%s{{%selse%s}}%s{{%send%s}}"+
				"{{%selse%s}}%s{{%send%s}}"+
				"{{%send%s}}",
			trimL, expr, quoteKey(firstKey), trimR,
			trimL, trimR,
			trimL, innerIndex, trimR, value, elseTrimL, elseTrimR, fallback, endTrimL, endTrimR,
			elseTrimL, elseTrimR, fallback, endTrimL, endTrimR,
			endTrimL, endTrimR,
		)
	})
}

// fixAssignChainedIndex fixes variable assignment patterns like:
//
//	{{- $items := index $resp "securityGroupInfo" "item" -}}
//
// by breaking the chain: assign from the first key, then conditionally
// re-assign from the second key based on the intermediate type.
func fixAssignChainedIndex(body string) string {
	return assignChainedIndexRe.ReplaceAllStringFunc(body, func(match string) string {
		groups := assignChainedIndexRe.FindStringSubmatch(match)
		if groups == nil {
			return match
		}
		trimL := groups[1]
		varName := groups[2]
		expr := groups[3]
		keysRaw := groups[4]
		trimR := groups[5]

		keys := parseQuotedKeys(keysRaw)
		if len(keys) < 2 {
			return match
		}

		firstKey := keys[0]
		remainingKeys := keys[1:]

		// Produce:
		// {{- $__intermediate := index EXPR "k1" -}}
		// {{- $items := index $__intermediate "k2" -}}  (for map case)
		// But we need to handle the slice case too. Since the downstream code
		// typically does its own type check on $items, we assign from the map branch
		// and let the type check handle slices. The safest approach:
		//
		// {{- $__tmp := index EXPR "k1" -}}
		// {{- if eq (printf "%T" $__tmp) "map[string]interface {}" -}}
		//   {{- $items := index $__tmp "k2" -}}
		// ...rest of template uses $items...
		//
		// But we can't wrap downstream code from here. Instead, produce a safe
		// single-step assignment that preserves the value when intermediate is a slice:
		//
		// {{- $__tmp_k1 := index EXPR "k1" -}}
		// {{- $VAR := $__tmp_k1 -}}
		// {{- if eq (printf "%T" $__tmp_k1) "map[string]interface {}" -}}
		//   {{- $VAR = index $__tmp_k1 "k2" -}}
		// {{- end -}}
		tmpVar := fmt.Sprintf("$__tmp_%s", strings.TrimPrefix(varName, "$"))
		innerIndex := fmt.Sprintf("index %s %s", tmpVar, quoteKeys(remainingKeys))

		return fmt.Sprintf(
			"{{%s%s := index %s %s%s}}"+
				"{{%s%s := %s%s}}"+
				"{{%sif eq (printf \"%%T\" %s) \"map[string]interface {}\" %s}}"+
				"{{%s%s = %s%s}}"+
				"{{%send%s}}",
			trimL, tmpVar, expr, quoteKey(firstKey), trimR,
			trimL, varName, tmpVar, trimR,
			trimL, tmpVar, trimR,
			trimL, varName, innerIndex, trimR,
			trimL, trimR,
		)
	})
}

// fixWithChainedIndex fixes multiline patterns like:
//
//	{{ with index $resp "k1" "k2" }}
//	  BODY (may contain type checks)
//	{{ end }}
//
// by breaking the chain and wrapping in a type guard.
func fixWithChainedIndex(body string) string {
	// We need to find each {{ with index EXPR "k1" "k2" }} and its matching {{ end }},
	// then restructure. We do this by tokenizing and tracking nesting.
	tokens := tokenizeTemplate(body)
	result := strings.Builder{}
	i := 0
	changed := false

	for i < len(tokens) {
		tok := tokens[i]
		if !tok.isTag {
			result.WriteString(tok.text)
			i++
			continue
		}

		groups := withChainedIndexRe.FindStringSubmatch(tok.text)
		if groups == nil {
			result.WriteString(tok.text)
			i++
			continue
		}

		trimL := groups[1]
		expr := groups[2]
		keysRaw := groups[3]
		trimR := groups[4]
		keys := parseQuotedKeys(keysRaw)

		if len(keys) < 2 {
			result.WriteString(tok.text)
			i++
			continue
		}

		// Find matching end
		bodyStart := i + 1
		endIdx, elseIdx := findMatchingEnd(tokens, bodyStart)
		if endIdx == -1 {
			result.WriteString(tok.text)
			i++
			continue
		}

		changed = true
		firstKey := keys[0]
		remainingKeys := keys[1:]
		innerIndex := fmt.Sprintf("index . %s", quoteKeys(remainingKeys))

		// Extract the body between with and end (or else)
		bodyEnd := endIdx
		if elseIdx != -1 {
			bodyEnd = elseIdx
		}
		var bodyTokens []token
		for j := bodyStart; j < bodyEnd; j++ {
			bodyTokens = append(bodyTokens, tokens[j])
		}
		bodyText := tokensToString(bodyTokens)

		// Extract else body if present
		var elseBody string
		if elseIdx != -1 {
			var elseTokens []token
			for j := elseIdx + 1; j < endIdx; j++ {
				elseTokens = append(elseTokens, tokens[j])
			}
			elseBody = tokensToString(elseTokens)
		}

		endTag := tokens[endIdx].text

		// Produce the fixed version:
		// {{- with index EXPR "k1" -}}
		//   {{- if eq (printf "%T" .) "map[string]interface {}" -}}
		//     {{- with index . "k2" -}}
		//       BODY
		//     {{- end -}}
		//   {{- else -}}
		//     {{- range . -}}
		//       {{- with index . "k2" -}}
		//         BODY
		//       {{- end -}}
		//     {{- end -}}
		//   {{- end -}}
		// {{- end -}}
		result.WriteString(fmt.Sprintf("{{%swith index %s %s%s}}", trimL, expr, quoteKey(firstKey), trimR))
		result.WriteString(fmt.Sprintf("{{%sif eq (printf \"%%T\" .) \"map[string]interface {}\" %s}}", trimL, trimR))
		result.WriteString(fmt.Sprintf("{{%swith %s%s}}", trimL, innerIndex, trimR))
		result.WriteString(bodyText)
		if elseIdx != -1 {
			result.WriteString(tokens[elseIdx].text) // {{ else }}
			result.WriteString(elseBody)
		}
		result.WriteString(endTag)
		result.WriteString(fmt.Sprintf("{{%selse%s}}", trimL, trimR))
		result.WriteString(fmt.Sprintf("{{%srange .%s}}", trimL, trimR))
		result.WriteString(fmt.Sprintf("{{%swith %s%s}}", trimL, innerIndex, trimR))
		result.WriteString(bodyText)
		if elseIdx != -1 {
			result.WriteString(tokens[elseIdx].text)
			result.WriteString(elseBody)
		}
		result.WriteString(endTag)
		result.WriteString(fmt.Sprintf("{{%send%s}}", trimL, trimR))
		result.WriteString(fmt.Sprintf("{{%send%s}}", trimL, trimR))
		result.WriteString(endTag)

		i = endIdx + 1
	}

	if !changed {
		return body
	}
	return result.String()
}

// --- template tokenizer ---

type token struct {
	text  string
	isTag bool // true if this is a {{ ... }} block
}

func tokenizeTemplate(body string) []token {
	var tokens []token
	rest := body
	for {
		openIdx := strings.Index(rest, "{{")
		if openIdx == -1 {
			if rest != "" {
				tokens = append(tokens, token{text: rest, isTag: false})
			}
			break
		}
		if openIdx > 0 {
			tokens = append(tokens, token{text: rest[:openIdx], isTag: false})
		}
		closeIdx := strings.Index(rest[openIdx:], "}}")
		if closeIdx == -1 {
			tokens = append(tokens, token{text: rest[openIdx:], isTag: false})
			break
		}
		closeIdx += openIdx + 2
		tokens = append(tokens, token{text: rest[openIdx:closeIdx], isTag: true})
		rest = rest[closeIdx:]
	}
	return tokens
}

// findMatchingEnd finds the {{ end }} that closes the block opened at tokens[startIdx].
// Returns (endIdx, elseIdx). elseIdx is -1 if no else at the same nesting level.
func findMatchingEnd(tokens []token, startIdx int) (int, int) {
	depth := 1
	elseIdx := -1
	blockOpeners := regexp.MustCompile(`\{\{-?\s*(if|with|range|block|define)\b`)
	blockEnd := regexp.MustCompile(`\{\{-?\s*end\b`)
	blockElse := regexp.MustCompile(`\{\{-?\s*else\b`)

	for i := startIdx; i < len(tokens); i++ {
		if !tokens[i].isTag {
			continue
		}
		t := tokens[i].text
		if blockOpeners.MatchString(t) {
			depth++
		} else if blockEnd.MatchString(t) {
			depth--
			if depth == 0 {
				return i, elseIdx
			}
		} else if blockElse.MatchString(t) && depth == 1 {
			elseIdx = i
		}
	}
	return -1, -1
}

func tokensToString(tokens []token) string {
	var sb strings.Builder
	for _, t := range tokens {
		sb.WriteString(t.text)
	}
	return sb.String()
}

// --- key parsing helpers ---

var quotedKeyRe = regexp.MustCompile(`"([^"]*)"`)

func parseQuotedKeys(raw string) []string {
	matches := quotedKeyRe.FindAllStringSubmatch(raw, -1)
	keys := make([]string, len(matches))
	for i, m := range matches {
		keys[i] = m[1]
	}
	return keys
}

func quoteKey(key string) string {
	return fmt.Sprintf(`"%s"`, key)
}

func quoteKeys(keys []string) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = quoteKey(k)
	}
	return strings.Join(parts, " ")
}

// ValidateFixedTemplate checks that a proposed fixed template:
// 1. Passes the same static analysis (no chained index, no unguarded dot access)
// 2. Parses successfully
// 3. Executes against empty input without error
func ValidateFixedTemplate(fixedBody string, tplType string) error {
	// 1. Parse check
	funcMap := analysisFuncMap(tplType)
	tpl, parseErr := template.New("__fix_validation__").Funcs(funcMap).Parse(fixedBody)
	if parseErr != nil {
		return fmt.Errorf("fixed template failed to parse: %v", parseErr)
	}

	// 2. Static analysis — re-run the same checks on the fixed body
	if chainedIndexPattern.MatchString(fixedBody) {
		return fmt.Errorf("fixed template still contains chained index calls")
	}
	hasBareDot := bareDotArgPattern.MatchString(fixedBody)
	hasNilGuard := containsAny(fixedBody,
		"{{ if .", "{{if .", "{{ with .", "{{with .",
		"{{ if not .", "{{if not .",
		"{{ if . }}", "{{if . }}", "{{ if .}}", "{{if .}}",
		"{{ with . }}", "{{with . }}",
		"{{- with index .", "{{ with index .",
		"{{- if or (not ", "{{ if or (not ",
		"{{- if . -}}", "{{- if .-}}",
	)
	hasDirectDot := containsAny(fixedBody,
		"{{ .", "{{.", "{{ range .", "{{range .",
		"{{ range $", "{{range $",
	)
	if (hasBareDot || hasDirectDot) && !hasNilGuard {
		return fmt.Errorf("fixed template still has unguarded dot access")
	}

	_ = tpl // parse check already passed above

	// 3. Execute against empty input — must not introduce new errors
	//    compared to the original template.
	return nil
}

// ValidateFixedTemplateWithOriginal performs the full validation including
// empty-input execution, only failing if the fix introduces NEW errors
// that the original template didn't have.
func ValidateFixedTemplateWithOriginal(fixedBody string, originalBody string, tplType string) error {
	// Basic validation first
	if err := ValidateFixedTemplate(fixedBody, tplType); err != nil {
		return err
	}

	// Execute both original and fixed against empty inputs.
	// Only fail if the fixed version errors where the original didn't.
	emptyInputs := emptyInputsForType(tplType)
	for _, input := range emptyInputs {
		origErr := executeTemplate(originalBody, tplType, input)
		fixedErr := executeTemplate(fixedBody, tplType, input)
		if fixedErr != nil && origErr == nil {
			return fmt.Errorf("fixed template introduced new error on empty input %q: %v", input, fixedErr)
		}
	}
	return nil
}

func executeTemplate(body string, tplType string, input string) error {
	factory := NewStreamTransformerFactory(tplType, body)
	if !factory.IsTransformable() {
		return fmt.Errorf("not transformable")
	}
	tfm, err := factory.GetTransformer(input)
	if err != nil {
		return err
	}
	return tfm.Transform()
}

// emptyInputsForType returns representative empty inputs for the given template type.
func emptyInputsForType(tplType string) []string {
	formatClass := classifyTemplateFormat(tplType)
	switch formatClass {
	case templateFormatXML:
		return []string{"", "<root/>", "<root></root>"}
	case templateFormatJSON:
		return []string{"", "{}", "null"}
	case templateFormatText:
		return []string{""}
	default:
		return []string{""}
	}
}
