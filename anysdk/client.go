package anysdk

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/stackql/any-sdk/pkg/auth_util"
	"github.com/stackql/any-sdk/pkg/client"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/pkg/internaldto"
	"github.com/stackql/any-sdk/pkg/netutils"
	"github.com/stackql/any-sdk/pkg/requesttranslate"
)

var (
	_ client.AnySdkClientConfigurator = &anySdkHTTPClientConfigurator{}
	_ client.AnySdkResponse           = &anySdkHttpResponse{}
	_ client.AnySdkClient             = &anySdkHttpClient{}
)

type anySdkHttpClient struct {
	client *http.Client
}

func newAnySdkHttpClient(client *http.Client) client.AnySdkClient {
	return &anySdkHttpClient{
		client: client,
	}
}

type anySdkHttpResponse struct {
	reponse *http.Response
}

func (hr *anySdkHttpResponse) IsErroneous() bool {
	return hr.reponse.StatusCode >= 400
}

func (hr *anySdkHttpResponse) GetHttpResponse() (*http.Response, error) {
	return hr.reponse, nil
}

func newAnySdkHttpReponse(httpResponse *http.Response) client.AnySdkResponse {
	return &anySdkHttpResponse{
		reponse: httpResponse,
	}
}

type anySdkArgList struct {
	args []client.AnySdkArg
}

func (al *anySdkArgList) GetArgs() []client.AnySdkArg {
	return al.args
}

func newAnySdkArgList(args ...client.AnySdkArg) client.AnySdkArgList {
	return &anySdkArgList{
		args: args,
	}
}

type anySdkHTTPArg struct {
	arg *http.Request
}

func (ha *anySdkHTTPArg) GetArg() (interface{}, bool) {
	return ha.arg, ha.arg != nil
}

func newAnySdkHTTPArg(arg *http.Request) client.AnySdkArg {
	return &anySdkHTTPArg{
		arg: arg,
	}
}

func (hc *anySdkHttpClient) Do(designation client.AnySdkDesignation, argList client.AnySdkArgList) (client.AnySdkResponse, error) {
	firstArg := argList.GetArgs()[0]
	arg, hasFirstArg := firstArg.GetArg()
	if !hasFirstArg {
		return nil, fmt.Errorf("could not get first argument")
	}
	httpReq, isHttpRequest := arg.(*http.Request)
	if !isHttpRequest {
		return nil, fmt.Errorf("could not cast first argument to http.Request")
	}
	httpResponse, httpResponseErr := hc.client.Do(httpReq)
	if httpResponseErr != nil {
		return nil, httpResponseErr
	}
	anySdkHttpResponse := newAnySdkHttpReponse(httpResponse)
	return anySdkHttpResponse, nil
}

type anySdkHTTPClientConfigurator struct {
	runtimeCtx   dto.RuntimeCtx
	authUtil     auth_util.AuthUtility
	providerName string
}

func NewAnySdkClientConfigurator(
	rtCtx dto.RuntimeCtx,
	provName string,
) client.AnySdkClientConfigurator {
	return &anySdkHTTPClientConfigurator{
		runtimeCtx:   rtCtx,
		authUtil:     auth_util.NewAuthUtility(),
		providerName: provName,
	}
}

func (cc *anySdkHTTPClientConfigurator) InferAuthType(authCtx dto.AuthCtx, authTypeRequested string) string {
	return cc.inferAuthType(authCtx, authTypeRequested)
}

func (cc *anySdkHTTPClientConfigurator) inferAuthType(authCtx dto.AuthCtx, authTypeRequested string) string {
	ft := strings.ToLower(authTypeRequested)
	switch ft {
	case dto.AuthAzureDefaultStr:
		return dto.AuthAzureDefaultStr
	case dto.AuthAPIKeyStr:
		return dto.AuthAPIKeyStr
	case dto.AuthBasicStr:
		return dto.AuthBasicStr
	case dto.AuthBearerStr:
		return dto.AuthBearerStr
	case dto.AuthServiceAccountStr:
		return dto.AuthServiceAccountStr
	case dto.AuthInteractiveStr:
		return dto.AuthInteractiveStr
	case dto.AuthNullStr:
		return dto.AuthNullStr
	case dto.AuthAWSSigningv4Str:
		return dto.AuthAWSSigningv4Str
	case dto.AuthCustomStr:
		return dto.AuthCustomStr
	case dto.OAuth2Str:
		return dto.OAuth2Str
	}
	if authCtx.KeyFilePath != "" || authCtx.KeyEnvVar != "" {
		return dto.AuthServiceAccountStr
	}
	return dto.AuthNullStr
}

func (cc *anySdkHTTPClientConfigurator) Auth(
	authCtx *dto.AuthCtx,
	authTypeRequested string,
	enforceRevokeFirst bool,
) (client.AnySdkClient, error) {
	authCtx = authCtx.Clone()
	at := cc.inferAuthType(*authCtx, authTypeRequested)
	switch at {
	case dto.AuthAPIKeyStr:
		httpClient, httpClientErr := cc.authUtil.ApiTokenAuth(authCtx, cc.runtimeCtx, false)
		if httpClientErr != nil {
			return nil, httpClientErr
		}
		return newAnySdkHttpClient(httpClient), nil
	case dto.AuthBearerStr:
		httpClient, httpClientErr := cc.authUtil.ApiTokenAuth(authCtx, cc.runtimeCtx, true)
		if httpClientErr != nil {
			return nil, httpClientErr
		}
		return newAnySdkHttpClient(httpClient), nil
	case dto.AuthServiceAccountStr:
		scopes := authCtx.Scopes
		httpClient, httpClientErr := cc.authUtil.GoogleOauthServiceAccount(cc.providerName, authCtx, scopes, cc.runtimeCtx)
		if httpClientErr != nil {
			return nil, httpClientErr
		}
		return newAnySdkHttpClient(httpClient), nil
	case dto.OAuth2Str:
		if authCtx.GrantType == dto.ClientCredentialsStr {
			scopes := authCtx.Scopes
			httpClient, httpClientErr := cc.authUtil.GenericOauthClientCredentials(authCtx, scopes, cc.runtimeCtx)
			if httpClientErr != nil {
				return nil, httpClientErr
			}
			return newAnySdkHttpClient(httpClient), nil
		}
	case dto.AuthBasicStr:
		httpClient, httpClientErr := cc.authUtil.BasicAuth(authCtx, cc.runtimeCtx)
		if httpClientErr != nil {
			return nil, httpClientErr
		}
		return newAnySdkHttpClient(httpClient), nil
	case dto.AuthCustomStr:
		httpClient, httpClientErr := cc.authUtil.CustomAuth(authCtx, cc.runtimeCtx)
		if httpClientErr != nil {
			return nil, httpClientErr
		}
		return newAnySdkHttpClient(httpClient), nil
	case dto.AuthAzureDefaultStr:
		httpClient, httpClientErr := cc.authUtil.AzureDefaultAuth(authCtx, cc.runtimeCtx)
		if httpClientErr != nil {
			return nil, httpClientErr
		}
		return newAnySdkHttpClient(httpClient), nil
	case dto.AuthInteractiveStr:
		httpClient, httpClientErr := cc.authUtil.GCloudOAuth(cc.runtimeCtx, authCtx, enforceRevokeFirst)
		if httpClientErr != nil {
			return nil, httpClientErr
		}
		return newAnySdkHttpClient(httpClient), nil
	case dto.AuthAWSSigningv4Str:
		httpClient, httpClientErr := cc.authUtil.AwsSigningAuth(authCtx, cc.runtimeCtx)
		if httpClientErr != nil {
			return nil, httpClientErr
		}
		return newAnySdkHttpClient(httpClient), nil
	case dto.AuthNullStr:
		httpClient := netutils.GetHTTPClient(cc.runtimeCtx, http.DefaultClient)
		return newAnySdkHttpClient(httpClient), nil
	}
	return nil, fmt.Errorf("could not infer auth type")
}

//nolint:nestif,mnd // acceptable for now
func parseReponseBodyIfErroneous(response *http.Response) (string, error) {
	if response != nil {
		if response.StatusCode >= 300 {
			if response.Body != nil {
				bodyBytes, bErr := io.ReadAll(response.Body)
				if bErr != nil {
					return "", bErr
				}
				bodyStr := string(bodyBytes)
				response.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				if len(bodyStr) > 0 {
					return fmt.Sprintf("http response status code: %d, response body: %s", response.StatusCode, bodyStr), nil
				}
			}
			return fmt.Sprintf("http response status code: %d, response body is nil", response.StatusCode), nil
		}
	}
	return "", nil
}

//nolint:nestif // acceptable for now
func parseReponseBodyIfPresent(response *http.Response) (string, error) {
	if response != nil {
		if response.Body != nil {
			bodyBytes, bErr := io.ReadAll(response.Body)
			if bErr != nil {
				return "", bErr
			}
			bodyStr := string(bodyBytes)
			response.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			if len(bodyStr) > 0 {
				return fmt.Sprintf("http response status code: %d, response body: %s", response.StatusCode, bodyStr), nil
			}
			return fmt.Sprintf("http response status code: %d, response body is nil", response.StatusCode), nil
		}
	}
	return "nil response", nil
}

type httpClientConfiguratorInput struct {
	authCtx            *dto.AuthCtx
	authType           string
	enforceRevokeFirst bool
}

func NewHttpClientConfiguratorInput(
	authCtx *dto.AuthCtx,
	authType string,
	enforceRevokeFirst bool,
) client.ClientConfiguratorInput {
	return &httpClientConfiguratorInput{
		authCtx:            authCtx,
		authType:           authType,
		enforceRevokeFirst: enforceRevokeFirst,
	}
}

func (hci *httpClientConfiguratorInput) GetAuthContext() *dto.AuthCtx {
	return hci.authCtx
}

func (hci *httpClientConfiguratorInput) GetAuthType() string {
	return hci.authType
}

func (hci *httpClientConfiguratorInput) GetEnforceRevokeFirst() bool {
	return hci.enforceRevokeFirst
}

type anySdkHTTPDesignation struct {
	method OperationStore
}

func NewAnySdkOpStoreDesignation(method OperationStore) client.AnySdkDesignation {
	return newAnySdkOpStoreDesignation(method)
}

func newAnySdkOpStoreDesignation(method OperationStore) client.AnySdkDesignation {
	return &anySdkHTTPDesignation{
		method: method,
	}
}

func (hd *anySdkHTTPDesignation) GetDesignation() (interface{}, bool) {
	return hd.method, hd.method != nil && reflect.TypeOf(hd.method) == reflect.TypeOf((*OperationStore)(nil))
}

func inferMaxResultsElement(OperationStore) internaldto.HTTPElement {
	return internaldto.NewHTTPElement(
		internaldto.QueryParam,
		"maxResults",
	)
}

func HTTPApiCallFromRequest(
	cc client.AnySdkClientConfigurator,
	runtimeCtx dto.RuntimeCtx,
	authCtx *dto.AuthCtx,
	authTypeRequested string,
	enforceRevokeFirst bool,
	outErrFile io.Writer,
	prov Provider,
	method OperationStore,
	request *http.Request,
) (*http.Response, error) {
	return httpApiCallFromRequest(
		cc,
		runtimeCtx,
		authCtx,
		authTypeRequested,
		enforceRevokeFirst,
		outErrFile,
		method,
		request,
	)
}

func CallFromSignature(
	cc client.AnySdkClientConfigurator,
	runtimeCtx dto.RuntimeCtx,
	authCtx *dto.AuthCtx,
	authTypeRequested string,
	enforceRevokeFirst bool,
	outErrFile io.Writer,
	prov Provider,
	designation client.AnySdkDesignation,
	argList client.AnySdkArgList,
) (client.AnySdkResponse, error) {
	rawDesignation, hasRawDesignation := designation.GetDesignation()
	if !hasRawDesignation {
		return nil, fmt.Errorf("could not get raw designation")
	}
	switch designation := rawDesignation.(type) {
	case *anySdkHTTPDesignation:
		method := designation.method
		firstArg := argList.GetArgs()[0]
		arg, hasFirstArg := firstArg.GetArg()
		if !hasFirstArg {
			return nil, fmt.Errorf("could not get first argument")
		}
		httpReq, isHttpRequest := arg.(*http.Request)
		if !isHttpRequest {
			return nil, fmt.Errorf("could not cast first argument to http.Request")
		}
		httpResponse, httpResponseErr := httpApiCallFromRequest(
			cc,
			runtimeCtx,
			authCtx,
			authTypeRequested,
			enforceRevokeFirst,
			outErrFile,
			method,
			httpReq,
		)
		if httpResponseErr != nil {
			return nil, httpResponseErr
		}
		anySdkHttpResponse := newAnySdkHttpReponse(httpResponse)
		return anySdkHttpResponse, nil
	default:
		return nil, fmt.Errorf("could not cast designation to anySdkHTTPDesignation")
	}
}

func httpApiCallFromRequest(
	cc client.AnySdkClientConfigurator,
	runtimeCtx dto.RuntimeCtx,
	authCtx *dto.AuthCtx,
	authTypeRequested string,
	enforceRevokeFirst bool,
	outErrFile io.Writer,
	method OperationStore,
	request *http.Request,
) (*http.Response, error) {
	httpClient, httpClientErr := cc.Auth(authCtx, authTypeRequested, enforceRevokeFirst)
	if httpClientErr != nil {
		return nil, httpClientErr
	}
	request.Header.Del("Authorization")
	requestTranslator, err := requesttranslate.NewRequestTranslator(method.GetRequestTranslateAlgorithm())
	if err != nil {
		return nil, err
	}
	translatedRequest, err := requestTranslator.Translate(request)
	if err != nil {
		return nil, err
	}
	if runtimeCtx.HTTPLogEnabled {
		urlStr := ""
		methodStr := ""
		if translatedRequest != nil && translatedRequest.URL != nil {
			urlStr = translatedRequest.URL.String()
			methodStr = translatedRequest.Method
		}
		//nolint:errcheck // output stream
		outErrFile.Write([]byte(fmt.Sprintf("http request url: '%s', method: '%s'\n", urlStr, methodStr)))
		body := translatedRequest.Body
		if body != nil {
			b, bErr := io.ReadAll(body)
			if bErr != nil {
				//nolint:errcheck // output stream
				outErrFile.Write([]byte(fmt.Sprintf("error inpecting http request body: %s\n", bErr.Error())))
			}
			bodyStr := string(b)
			translatedRequest.Body = io.NopCloser(bytes.NewBuffer(b))
			//nolint:errcheck // output stream
			outErrFile.Write([]byte(fmt.Sprintf("http request body = '%s'\n", bodyStr)))
		}
	}
	r, err := httpClient.Do(
		newAnySdkOpStoreDesignation(method),
		newAnySdkArgList(
			newAnySdkHTTPArg(translatedRequest),
		),
	)
	if err != nil {
		return nil, err
	}
	httpResponse, _ := r.GetHttpResponse()
	responseErrorBodyToPublish, reponseParseErr := parseReponseBodyIfErroneous(httpResponse)
	if reponseParseErr != nil {
		return nil, reponseParseErr
	}
	if responseErrorBodyToPublish != "" {
		//nolint:errcheck // output stream
		outErrFile.Write([]byte(fmt.Sprintf("%s\n", responseErrorBodyToPublish)))
	} else if runtimeCtx.HTTPLogEnabled {
		reponseBodyStr, _ := parseReponseBodyIfPresent(httpResponse)
		//nolint:errcheck // output stream
		outErrFile.Write([]byte(fmt.Sprintf("%s\n", reponseBodyStr)))
	}
	if err != nil {
		if runtimeCtx.HTTPLogEnabled {
			//nolint:errcheck // output stream
			outErrFile.Write([]byte(
				fmt.Sprintln(fmt.Sprintf("http response error: %s", err.Error()))), //nolint:gosimple,lll // TODO: sweep through this sort of nonsense
			)
		}
		return nil, err
	}
	return httpResponse, err
}
