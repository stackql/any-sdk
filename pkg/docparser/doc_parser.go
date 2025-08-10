package docparser

import (
	"strings"
)

const (
	SchemaDelimiter            string = "."
	googleServiceKeyDelimiter  string = ":"
	stackqlServiceKeyDelimiter string = "__"
)

func TranslateServiceKeyGenericProviderToIql(serviceKey string) string {
	//nolint:gocritic // TODO: review
	return strings.Replace(serviceKey, googleServiceKeyDelimiter, stackqlServiceKeyDelimiter, -1)
}
