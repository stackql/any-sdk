package discovery

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/stackql/any-sdk/internal/anysdk"
)

// Analysis bins for new static checks.
const (
	BinRequestParamUnroutable   = "request-param-unroutable"
	BinRefResolutionFailed      = "ref-resolution-failed"
	BinSQLVerbCoverage          = "sql-verb-coverage"
	BinServerURLInvalid         = "server-url-invalid"
	BinPaginationIncomplete     = "pagination-incomplete"
	BinTransformSchemaMismatch  = "transform-schema-mismatch"
)

// checkRequestParamRoutability validates that required parameters have
// valid locations and can be routed to the HTTP request.
func checkRequestParamRoutability(
	actx AnalysisContext,
	method anysdk.StandardOperationStore,
) []AnalysisFinding {
	var findings []AnalysisFinding
	params := method.GetRequiredParameters()
	for key, param := range params {
		if param == nil {
			findings = append(findings, actx.NewWarning(BinRequestParamUnroutable,
				fmt.Sprintf("required parameter '%s' is nil", key)))
			continue
		}
		loc := param.GetLocation()
		if loc == "" {
			findings = append(findings, actx.NewWarning(BinRequestParamUnroutable,
				fmt.Sprintf("required parameter '%s' has no location", key)))
		}
	}
	return findings
}

// checkRefResolution validates that the operation ref resolves to a non-nil operation.
func checkRefResolution(
	actx AnalysisContext,
	method anysdk.StandardOperationStore,
) []AnalysisFinding {
	var findings []AnalysisFinding
	opRef := method.GetOperationRef()
	if opRef == nil {
		findings = append(findings, actx.NewError(BinRefResolutionFailed,
			"operation ref is nil"))
		return findings
	}
	if opRef.Ref == "" && len(opRef.GetInline()) == 0 {
		findings = append(findings, actx.NewError(BinRefResolutionFailed,
			"operation ref has no $ref and no inline"))
	}
	// Check response schema ref resolves
	resp, hasResp := method.GetResponse()
	if hasResp {
		schema := resp.GetSchema()
		rawSchema := resp.GetRawSchema()
		if schema == nil && rawSchema == nil {
			findings = append(findings, actx.NewWarning(BinRefResolutionFailed,
				"response has no resolved schema (raw or override)"))
		}
	}
	return findings
}

// checkSQLVerbCoverage validates that a resource has at least a SELECT method
// and that non-SELECT resources have appropriate parameters.
func checkSQLVerbCoverage(
	actx AnalysisContext,
	resource anysdk.Resource,
) []AnalysisFinding {
	var findings []AnalysisFinding
	methods := resource.GetMethods()
	if len(methods) == 0 {
		findings = append(findings, AnalysisFinding{
			Level:    "warning",
			Bin:      BinSQLVerbCoverage,
			Provider: actx.Provider,
			Service:  actx.Service,
			Resource: actx.Resource,
			Message:  "resource has no methods",
		})
		return findings
	}

	hasSelect := false
	for _, m := range methods {
		if strings.ToLower(m.GetSQLVerb()) == "select" {
			hasSelect = true
			break
		}
	}
	if !hasSelect {
		findings = append(findings, AnalysisFinding{
			Level:    "warning",
			Bin:      BinSQLVerbCoverage,
			Provider: actx.Provider,
			Service:  actx.Service,
			Resource: actx.Resource,
			Message:  "resource has no SELECT method",
		})
	}
	return findings
}

// checkServerURLValidity validates server URL templates are well-formed
// and that template variables have defaults.
func checkServerURLValidity(
	actx AnalysisContext,
	method anysdk.StandardOperationStore,
) []AnalysisFinding {
	var findings []AnalysisFinding
	servers, hasServers := method.GetServers()
	if !hasServers || len(servers) == 0 {
		findings = append(findings, actx.NewWarning(BinServerURLInvalid,
			"method has no server definitions"))
		return findings
	}
	for _, srv := range servers {
		if srv == nil || srv.URL == "" {
			findings = append(findings, actx.NewWarning(BinServerURLInvalid,
				"server entry has empty URL"))
			continue
		}
		// Check URL is parseable (ignoring template vars)
		testURL := srv.URL
		// Replace {var} with placeholder for URL parsing
		for varName := range srv.Variables {
			testURL = strings.ReplaceAll(testURL, "{"+varName+"}", "placeholder")
		}
		if _, err := url.Parse(testURL); err != nil {
			findings = append(findings, actx.NewWarning(BinServerURLInvalid,
				fmt.Sprintf("server URL '%s' is not valid: %v", srv.URL, err)))
		}
		// Check template variables have defaults
		for varName, varDef := range srv.Variables {
			if varDef == nil {
				findings = append(findings, actx.NewWarning(BinServerURLInvalid,
					fmt.Sprintf("server variable '%s' has nil definition", varName)))
				continue
			}
			if varDef.Default == "" {
				findings = append(findings, actx.NewWarning(BinServerURLInvalid,
					fmt.Sprintf("server variable '%s' has no default value", varName)))
			}
		}
	}
	return findings
}

// checkPaginationCompleteness validates that pagination config has both
// request and response token semantics with valid keys.
func checkPaginationCompleteness(
	actx AnalysisContext,
	method anysdk.StandardOperationStore,
) []AnalysisFinding {
	var findings []AnalysisFinding
	reqToken, hasReqToken := method.GetPaginationRequestTokenSemantic()
	respToken, hasRespToken := method.GetPaginationResponseTokenSemantic()

	// Only check if at least one token is configured (method claims to support pagination)
	if !hasReqToken && !hasRespToken {
		return nil
	}

	if hasReqToken && !hasRespToken {
		findings = append(findings, actx.NewWarning(BinPaginationIncomplete,
			"pagination has request token but no response token"))
	}
	if hasRespToken && !hasReqToken {
		findings = append(findings, actx.NewWarning(BinPaginationIncomplete,
			"pagination has response token but no request token"))
	}
	if hasReqToken && reqToken.GetKey() == "" {
		findings = append(findings, actx.NewWarning(BinPaginationIncomplete,
			"pagination request token has empty key"))
	}
	if hasRespToken && respToken.GetKey() == "" {
		findings = append(findings, actx.NewWarning(BinPaginationIncomplete,
			"pagination response token has empty key"))
	}
	return findings
}

// BinCrossResourceInconsistency is the bin for cross-resource consistency issues.
const BinCrossResourceInconsistency = "cross-resource-inconsistent"

// checkCrossResourceConsistency checks that _list_only resources have a
// corresponding parent resource in the same service.
func checkCrossResourceConsistency(
	actx AnalysisContext,
	resourceKey string,
	allResources map[string]anysdk.Resource,
) []AnalysisFinding {
	var findings []AnalysisFinding
	if !strings.HasSuffix(resourceKey, "_list_only") {
		return nil
	}
	parentKey := strings.TrimSuffix(resourceKey, "_list_only")
	if _, ok := allResources[parentKey]; !ok {
		findings = append(findings, AnalysisFinding{
			Level:    "warning",
			Bin:      BinCrossResourceInconsistency,
			Provider: actx.Provider,
			Service:  actx.Service,
			Resource: actx.Resource,
			Message:  fmt.Sprintf("list_only resource '%s' has no corresponding parent resource '%s'", resourceKey, parentKey),
		})
	}
	return findings
}

// checkTransformSchemaConsistency validates that when a response has both
// a transform and a schema_override, the transform output type is consistent
// with the override media type.
func checkTransformSchemaConsistency(
	actx AnalysisContext,
	method anysdk.StandardOperationStore,
) []AnalysisFinding {
	var findings []AnalysisFinding
	resp, hasResp := method.GetResponse()
	if !hasResp {
		return nil
	}
	transform, hasTransform := resp.GetTransform()
	if !hasTransform {
		return nil
	}
	overrideMediaType := resp.GetOverrrideBodyMediaType()
	rawMediaType := resp.GetBodyMediaType()

	// If there's a transform and override media type, check consistency
	if overrideMediaType != "" && rawMediaType != "" && overrideMediaType != rawMediaType {
		// Transform changes media type — check the transform type matches the output
		tplType := transform.GetType()
		if strings.Contains(tplType, "mxj") && !strings.Contains(overrideMediaType, "json") {
			findings = append(findings, actx.NewWarning(BinTransformSchemaMismatch,
				fmt.Sprintf("MXJ transform type '%s' but override media type is '%s' (expected application/json)", tplType, overrideMediaType)))
		}
	}

	// If there's a schema_override, check the override schema is non-nil
	overrideSchema := resp.GetSchema()
	if overrideMediaType != "" && overrideSchema == nil {
		findings = append(findings, actx.NewWarning(BinTransformSchemaMismatch,
			"response has override media type but no resolved override schema"))
	}

	// Check transform body is non-empty
	if transform.GetBody() == "" {
		findings = append(findings, actx.NewWarning(BinTransformSchemaMismatch,
			"response transform has empty body"))
	}

	return findings
}
