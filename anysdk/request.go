package anysdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/stackql/any-sdk/pkg/streaming"
)

type HTTPPreparator interface {
	BuildHTTPRequestCtx() (HTTPArmoury, error)
	BuildHTTPRequestCtxFromAnnotation() (HTTPArmoury, error)
}

type standardHTTPPreparator struct {
	prov        Provider
	m           OperationStore
	svc         Service
	paramMap    map[int]map[string]interface{}
	execContext ExecContext
	logger      *logrus.Logger
	parameters  streaming.MapStream
}

func NewHTTPPreparator(
	prov Provider,
	svc Service,
	m OperationStore,
	paramMap map[int]map[string]interface{},
	parameters streaming.MapStream,
	execContext ExecContext,
	logger *logrus.Logger,
) HTTPPreparator {
	return newHTTPPreparator(prov, svc, m, paramMap, parameters, execContext, logger)
}

func newHTTPPreparator(
	prov Provider,
	svc Service,
	m OperationStore,
	paramMap map[int]map[string]interface{},
	parameters streaming.MapStream,
	execContext ExecContext,
	logger *logrus.Logger,
) HTTPPreparator {
	if logger == nil {
		logger = logrus.New()
		logger.SetOutput(io.Discard)
	}
	return &standardHTTPPreparator{
		prov:        prov,
		m:           m,
		svc:         svc,
		paramMap:    paramMap,
		parameters:  parameters,
		execContext: execContext,
		logger:      logger,
	}
}

//nolint:funlen,gocognit // TODO: review
func (pr *standardHTTPPreparator) BuildHTTPRequestCtx() (HTTPArmoury, error) {
	var err error
	httpArmoury := NewHTTPArmoury()
	var requestSchema, responseSchema Schema
	req, reqExists := pr.m.GetRequest()
	if reqExists && req.GetSchema() != nil {
		requestSchema = req.GetSchema()
	}
	res, resExists := pr.m.GetResponse()
	if resExists && res.GetSchema() != nil {
		responseSchema = res.GetSchema()
	}
	httpArmoury.SetRequestSchema(requestSchema)
	httpArmoury.SetResponseSchema(responseSchema)
	paramList, err := splitHTTPParameters(pr.paramMap, pr.m)
	if err != nil {
		return nil, err
	}
	//nolint:dupl // TODO: review
	for _, prms := range paramList {
		params := prms
		pm := NewHTTPArmouryParameters()
		if pr.execContext != nil && pr.execContext.GetExecPayload() != nil {
			pm.SetBodyBytes(pr.execContext.GetExecPayload().GetPayload())
			for j, v := range pr.execContext.GetExecPayload().GetHeader() {
				pm.SetHeaderKV(j, v)
			}
			params.SetRequestBody(pr.execContext.GetExecPayload().GetPayloadMap())
		} else if params.GetRequestBody() != nil && len(params.GetRequestBody()) != 0 {
			m := make(map[string]interface{})
			baseRequestBytes := pr.m.getBaseRequestBodyBytes()
			if len(baseRequestBytes) > 0 {
				mapErr := json.Unmarshal(baseRequestBytes, &m)
				if mapErr != nil {
					return nil, fmt.Errorf("error unmarshalling base request: %v", mapErr)
				}
			}
			for k, v := range params.GetRequestBody() {
				m[k] = v
			}
			b, bErr := json.Marshal(m)
			if bErr != nil {
				return nil, bErr
			}
			pm.SetBodyBytes(b)
			req, reqExists := pr.m.GetRequest() //nolint:govet // intentional shadowing
			if reqExists {
				pm.SetHeaderKV("Content-Type", []string{req.GetBodyMediaType()})
			}
		} else if len(pr.m.getDefaultRequestBodyBytes()) > 0 {
			pm.SetBodyBytes(pr.m.getDefaultRequestBodyBytes())
			req, reqExists := pr.m.GetRequest() //nolint:govet // intentional shadowing
			if reqExists {
				pm.SetHeaderKV("Content-Type", []string{req.GetBodyMediaType()})
			}
		}
		resp, respExists := pr.m.GetResponse()
		if respExists {
			if resp.GetBodyMediaType() != "" && pr.prov.GetName() != "aws" {
				pm.SetHeaderKV("Accept", []string{resp.GetBodyMediaType()})
			}
		}
		pm.SetParameters(params)
		httpArmoury.AddRequestParams(pm)
	}
	secondPassParams := httpArmoury.GetRequestParams()
	for i, param := range secondPassParams {
		p := param
		if len(p.GetParameters().GetRequestBody()) == 0 && len(pr.m.getDefaultRequestBodyBytes()) == 0 {
			p.SetRequestBodyMap(nil)
		} else if len(pr.m.getDefaultRequestBodyBytes()) > 0 && len(p.GetParameters().GetRequestBody()) == 0 {
			bm := make(map[string]interface{})
			// TODO: support types other than json
			err := json.Unmarshal(pr.m.getDefaultRequestBodyBytes(), &bm)
			if err == nil {
				p.SetRequestBodyMap(bm)
			}
		}
		var baseRequestCtx *http.Request
		baseRequestCtx, err = getRequest(pr.prov, pr.svc, pr.m, p.GetParameters())
		if err != nil {
			return nil, err
		}
		for k, v := range p.GetHeader() {
			for _, vi := range v {
				baseRequestCtx.Header.Set(k, vi)
			}
		}
		p.SetRequest(baseRequestCtx)
		pr.logger.Infoln(
			fmt.Sprintf(
				"pre transform: httpArmoury.RequestParams[%d] = %s", i, string(p.GetBodyBytes())))
		pr.logger.Infoln(
			fmt.Sprintf(
				"post transform: httpArmoury.RequestParams[%d] = %s", i, string(p.GetBodyBytes())))
		secondPassParams[i] = p
	}
	httpArmoury.SetRequestParams(secondPassParams)
	if err != nil {
		return nil, err
	}
	return httpArmoury, nil
}

func awsContextHousekeeping(
	ctx context.Context,
	method OperationStore,
	parameters map[string]interface{},
) context.Context {
	svcName := method.getServiceNameForProvider()
	ctx = context.WithValue(ctx, "service", svcName) //nolint:revive,staticcheck // TODO: add custom context type
	if region, ok := parameters["region"]; ok {
		if regionStr, rOk := region.(string); rOk {
			ctx = context.WithValue(ctx, "region", regionStr) //nolint:revive,staticcheck // TODO: add custom context type
		}
	}
	return ctx
}

func getRequest(
	prov Provider,
	svc Service,
	method OperationStore,
	httpParams HttpParameters,
) (*http.Request, error) {
	params, err := httpParams.ToFlatMap()
	if err != nil {
		return nil, err
	}
	validationParams, err := method.Parameterize(prov, svc, httpParams, httpParams.GetRequestBody())
	if err != nil {
		return nil, err
	}
	request := validationParams.Request
	ctx := awsContextHousekeeping(request.Context(), method, params)
	request = request.WithContext(ctx)
	return request, nil
}

//nolint:funlen,gocognit // acceptable
func (pr *standardHTTPPreparator) BuildHTTPRequestCtxFromAnnotation() (HTTPArmoury, error) {
	var err error
	httpArmoury := NewHTTPArmoury()
	var requestSchema, responseSchema Schema
	req, reqExists := pr.m.GetRequest()
	if reqExists && req.GetSchema() != nil {
		requestSchema = req.GetSchema()
	}
	resp, respExists := pr.m.GetResponse()
	if respExists && resp.GetSchema() != nil {
		responseSchema = resp.GetSchema()
	}
	httpArmoury.SetRequestSchema(requestSchema)
	httpArmoury.SetResponseSchema(responseSchema)

	paramMap := make(map[int]map[string]interface{})
	i := 0
	for {
		out, oErr := pr.parameters.Read()
		for _, m := range out {
			paramMap[i] = m
			i++
		}
		if errors.Is(oErr, io.EOF) {
			break
		}
		if oErr != nil {
			return nil, oErr
		}
	}
	paramList, err := splitHTTPParameters(paramMap, pr.m)
	if err != nil {
		return nil, err
	}
	for _, prms := range paramList { //nolint:dupl // TODO: refactor
		params := prms
		pm := NewHTTPArmouryParameters()
		if pr.execContext != nil && pr.execContext.GetExecPayload() != nil {
			pm.SetBodyBytes(pr.execContext.GetExecPayload().GetPayload())
			for j, v := range pr.execContext.GetExecPayload().GetHeader() {
				pm.SetHeaderKV(j, v)
			}
			params.SetRequestBody(pr.execContext.GetExecPayload().GetPayloadMap())
		} else if params.GetRequestBody() != nil && len(params.GetRequestBody()) != 0 {
			m := make(map[string]interface{})
			baseRequestBytes := pr.m.getBaseRequestBodyBytes()
			if len(baseRequestBytes) > 0 {
				mapErr := json.Unmarshal(baseRequestBytes, &m)
				if mapErr != nil {
					return nil, fmt.Errorf("error unmarshalling base request: %v", mapErr)
				}
			}
			for k, v := range params.GetRequestBody() {
				m[k] = v
			}
			b, jErr := json.Marshal(m)
			if jErr != nil {
				return nil, jErr
			}
			pm.SetBodyBytes(b)
			req, reqExists := pr.m.GetRequest() //nolint:govet // intentional
			if reqExists {
				pm.SetHeaderKV("Content-Type", []string{req.GetBodyMediaType()})
			}
		} else if len(pr.m.getDefaultRequestBodyBytes()) > 0 {
			pm.SetBodyBytes(pr.m.getDefaultRequestBodyBytes())
			req, reqExists := pr.m.GetRequest() //nolint:govet // intentional shadowing
			if reqExists {
				pm.SetHeaderKV("Content-Type", []string{req.GetBodyMediaType()})
			}
		}
		resp, respExists := pr.m.GetResponse() //nolint:govet // intentional
		if respExists {
			if resp.GetBodyMediaType() != "" && pr.prov.GetName() != "aws" {
				pm.SetHeaderKV("Accept", []string{resp.GetBodyMediaType()})
			}
		}
		pm.SetParameters(params)
		httpArmoury.AddRequestParams(pm)
	}
	secondPassParams := httpArmoury.GetRequestParams()
	for i, param := range secondPassParams {
		p := param
		if len(p.GetParameters().GetRequestBody()) == 0 && len(pr.m.getDefaultRequestBodyBytes()) == 0 {
			p.SetRequestBodyMap(nil)
		} else if len(pr.m.getDefaultRequestBodyBytes()) > 0 {
			bm := make(map[string]interface{})
			// TODO: support types other than json
			err := json.Unmarshal(pr.m.getDefaultRequestBodyBytes(), &bm)
			if err == nil {
				p.SetRequestBodyMap(bm)
			}
		}
		var baseRequestCtx *http.Request
		baseRequestCtx, err = getRequest(pr.prov, pr.svc, pr.m, p.GetParameters())
		if err != nil {
			return nil, err
		}
		for k, v := range p.GetHeader() {
			for _, vi := range v {
				baseRequestCtx.Header.Set(k, vi)
			}
		}

		p.SetRequest(baseRequestCtx)
		pr.logger.Infoln(
			fmt.Sprintf("pre transform: httpArmoury.RequestParams[%d] = %s",
				i, string(p.GetBodyBytes())))
		pr.logger.Infoln(
			fmt.Sprintf("post transform: httpArmoury.RequestParams[%d] = %s",
				i, string(p.GetBodyBytes())))
		secondPassParams[i] = p
	}
	httpArmoury.SetRequestParams(secondPassParams)
	if err != nil {
		return nil, err
	}
	return httpArmoury, nil
}
