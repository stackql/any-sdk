package anysdk

import (
	"fmt"
	"regexp"

	"github.com/stackql/any-sdk/pkg/fuzzymatch"
)

const (
	MethodDescription string = "description"
	MethodName        string = "MethodName"
	RequiredParams    string = "RequiredParams"
	SQLVerb           string = "SQLVerb"
)

const (
	ExtensionKeyAlwaysRequired string = "x-alwaysRequired"
	ExtensionKeyGraphQL        string = "x-stackQL-graphQL"
	ExtensionKeyConfig         string = "x-stackQL-config"
	ExtensionKeyProvider       string = "x-stackql-provider"
	ExtensionKeyResources      string = "x-stackQL-resources"
	ExtensionKeyStringOnly     string = "x-stackQL-stringOnly"
)

const (
	requestBodyKeyPrefix    string = "data"
	requestBodyKeyDelimiter string = "__"
	requestBodyBaseKey      string = requestBodyKeyPrefix + requestBodyKeyDelimiter
)

const (
	ViewKeyResourceLevelSelect string = "select"
)

var (
	requestBodyBaseKeyRegexp       *regexp.Regexp                  = regexp.MustCompile(fmt.Sprintf("^%s%s", requestBodyBaseKey, "(.*)"))
	requestBodyBaseKeyFuzzyMatcher fuzzymatch.FuzzyMatcher[string] = fuzzymatch.NewRegexpStringMetcher([]fuzzymatch.StringFuzzyPair{
		fuzzymatch.NewFuzzyPair(requestBodyBaseKeyRegexp, requestBodyBaseKey),
	})
)
