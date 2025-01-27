package argparse

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/pprof"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/auth_util"
	"github.com/stackql/any-sdk/pkg/constants"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/pkg/internaldto"
	"github.com/stackql/any-sdk/pkg/netutils"
)

type genericProvider struct {
	provider   anysdk.Provider
	runtimeCtx dto.RuntimeCtx
	authUtil   auth_util.AuthUtility
}

func newGenericProvider(rtCtx dto.RuntimeCtx, prov anysdk.Provider) *genericProvider {
	return &genericProvider{
		runtimeCtx: rtCtx,
		authUtil:   auth_util.NewAuthUtility(),
		provider:   prov,
	}
}

func (gp *genericProvider) inferAuthType(authCtx dto.AuthCtx, authTypeRequested string) string {
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

func (gp *genericProvider) Auth(
	authCtx *dto.AuthCtx,
	authTypeRequested string,
	enforceRevokeFirst bool,
) (*http.Client, error) {
	authCtx = authCtx.Clone()
	at := gp.inferAuthType(*authCtx, authTypeRequested)
	switch at {
	case dto.AuthAPIKeyStr:
		return gp.authUtil.ApiTokenAuth(authCtx, gp.runtimeCtx, false)
	case dto.AuthBearerStr:
		return gp.authUtil.ApiTokenAuth(authCtx, gp.runtimeCtx, true)
	case dto.AuthServiceAccountStr:
		scopes := authCtx.Scopes
		return gp.authUtil.GoogleOauthServiceAccount(gp.provider.GetName(), authCtx, scopes, gp.runtimeCtx)
	case dto.OAuth2Str:
		if authCtx.GrantType == dto.ClientCredentialsStr {
			scopes := authCtx.Scopes
			return gp.authUtil.GenericOauthClientCredentials(authCtx, scopes, gp.runtimeCtx)
		}
	case dto.AuthBasicStr:
		return gp.authUtil.BasicAuth(authCtx, gp.runtimeCtx)
	case dto.AuthCustomStr:
		return gp.authUtil.CustomAuth(authCtx, gp.runtimeCtx)
	case dto.AuthAzureDefaultStr:
		return gp.authUtil.AzureDefaultAuth(authCtx, gp.runtimeCtx)
	case dto.AuthInteractiveStr:
		return gp.authUtil.GCloudOAuth(gp.runtimeCtx, authCtx, enforceRevokeFirst)
	case dto.AuthAWSSigningv4Str:
		return gp.authUtil.AwsSigningAuth(authCtx, gp.runtimeCtx)
	case dto.AuthNullStr:
		return netutils.GetHTTPClient(gp.runtimeCtx, http.DefaultClient), nil
	}
	return nil, fmt.Errorf("could not infer auth type")
}

func getLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	return logger
}

func parseExecPayload(
	payload string,
	payloadType string,
) (internaldto.ExecPayload, error) {
	payloadBytes := []byte(payload)
	m := make(map[string][]string)
	pm := map[string]interface{}{}
	switch payloadType {
	case constants.JSONStr, "application/json":
		if len(payloadBytes) > 0 {
			m["Content-Type"] = []string{"application/json"}
			err := json.Unmarshal(payloadBytes, &pm)
			if err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("payload map of declared type = '%T' not allowed", payloadType)
	}
	return internaldto.NewExecPayload(
		payloadBytes,
		m,
		pm,
	), nil
}

type queryCmdPayload struct {
	rtCtx        dto.RuntimeCtx
	provFilePath string
	svcFilePath  string
	resourceStr  string
	methodName   string
	payload      string
	payloadType  string
	parameters   map[string]interface{}
	auth         map[string]*dto.AuthCtx
}

func (qcp *queryCmdPayload) getService() (anysdk.Service, error) {
	b, err := os.ReadFile(qcp.svcFilePath)
	if err != nil {
		return nil, err
	}
	l := anysdk.NewLoader()
	svc, err := l.LoadFromBytes(b)
	if err != nil {
		return nil, err
	}
	pb, err := os.ReadFile(qcp.provFilePath)
	if err != nil {
		return nil, err
	}
	prov, err := anysdk.LoadProviderDocFromBytes(pb)
	if err != nil {
		return nil, err
	}
	svc.SetProvider(prov)
	return svc, nil
}

func newQueryCmdPayload(rtCtx dto.RuntimeCtx) (*queryCmdPayload, error) {
	var params map[string]interface{}
	err := json.Unmarshal([]byte(rtCtx.CLIParameters), &params)
	if err != nil {
		return nil, err
	}
	ac := make(map[string]*dto.AuthCtx)
	err = yaml.Unmarshal([]byte(runtimeCtx.AuthRaw), ac)
	if err != nil {
		return nil, err
	}
	return &queryCmdPayload{
		rtCtx:        rtCtx,
		svcFilePath:  rtCtx.CLISvcFilePath,
		provFilePath: rtCtx.CLIProvFilePath,
		resourceStr:  rtCtx.CLIResourceStr,
		methodName:   rtCtx.CLIMethodName,
		payload:      rtCtx.CLIPayload,
		payloadType:  rtCtx.CLIPayloadType,
		parameters:   params,
		auth:         ac,
	}, nil
}

func runQueryCommand(gp *genericProvider, authCtx *dto.AuthCtx, payload *queryCmdPayload) error {
	svc, err := payload.getService()
	if err != nil {
		return err
	}
	res, err := svc.GetResource(payload.resourceStr)
	if err != nil {
		return err
	}
	opStore, err := res.FindMethod(payload.methodName)
	if err != nil {
		return err
	}
	execPayload, err := parseExecPayload(
		payload.payload,
		payload.payloadType,
	)
	if err != nil {
		return err
	}
	execCtx := anysdk.NewExecContext(
		execPayload,
		res,
	)
	prep := anysdk.NewHTTPPreparator(
		svc.GetProvider(),
		svc,
		opStore,
		map[int]map[string]interface{}{
			0: payload.parameters,
		},
		nil,
		execCtx,
		getLogger(),
	)
	httpClient, err := gp.Auth(
		authCtx,
		authCtx.Type,
		false,
	)
	if err != nil {
		return err
	}
	armoury, err := prep.BuildHTTPRequestCtx()
	if err != nil {
		return err
	}
	for _, v := range armoury.GetRequestParams() {
		req := v.GetRequest()
		response, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "response = '%s'\n", string(bodyBytes))
	}
	return nil
}

func transformOpenapiStackqlAuthToLocal(authDTO anysdk.AuthDTO) *dto.AuthCtx {
	rv := &dto.AuthCtx{
		Scopes:                  authDTO.GetScopes(),
		Subject:                 authDTO.GetSubject(),
		Type:                    authDTO.GetType(),
		ValuePrefix:             authDTO.GetValuePrefix(),
		KeyID:                   authDTO.GetKeyID(),
		KeyIDEnvVar:             authDTO.GetKeyIDEnvVar(),
		KeyFilePath:             authDTO.GetKeyFilePath(),
		KeyFilePathEnvVar:       authDTO.GetKeyFilePathEnvVar(),
		KeyEnvVar:               authDTO.GetKeyEnvVar(),
		EnvVarAPIKeyStr:         authDTO.GetEnvVarAPIKeyStr(),
		EnvVarAPISecretStr:      authDTO.GetEnvVarAPISecretStr(),
		EnvVarUsername:          authDTO.GetEnvVarUsername(),
		EnvVarPassword:          authDTO.GetEnvVarPassword(),
		EncodedBasicCredentials: authDTO.GetInlineBasicCredentials(),
		Location:                authDTO.GetLocation(),
		Name:                    authDTO.GetName(),
		TokenURL:                authDTO.GetTokenURL(),
		GrantType:               authDTO.GetGrantType(),
		ClientID:                authDTO.GetClientID(),
		ClientSecret:            authDTO.GetClientSecret(),
		ClientIDEnvVar:          authDTO.GetClientIDEnvVar(),
		ClientSecretEnvVar:      authDTO.GetClientSecretEnvVar(),
		Values:                  authDTO.GetValues(),
		AuthStyle:               authDTO.GetAuthStyle(),
		AccountID:               authDTO.GetAccountID(),
		AccoountIDEnvVar:        authDTO.GetAccountIDEnvVar(),
	}
	successor, successorExists := authDTO.GetSuccessor()
	currentParent := rv
	for {
		if successorExists {
			transformedSuccessor := transformOpenapiStackqlAuthToLocal(successor)
			currentParent.Successor = transformedSuccessor
			currentParent = transformedSuccessor
			successor, successorExists = successor.GetSuccessor()
		} else {
			break
		}
	}
	return rv
}

// queryCmd represents the query command
var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Simple provider query",
	Long:  `Simple provider query`,
	Run: func(cmd *cobra.Command, args []string) {

		if runtimeCtx.CPUProfile != "" {
			f, err := os.Create(runtimeCtx.CPUProfile)
			if err != nil {
				printErrorAndExitOneIfError(err)
			}
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		}

		// if len(args) == 0 || args[0] == "" {
		// 	cmd.Help()
		// 	os.Exit(0)
		// }

		payload, err := newQueryCmdPayload(runtimeCtx)

		printErrorAndExitOneIfError(err)

		svc, err := payload.getService()

		provStr := svc.GetProvider().GetName()

		printErrorAndExitOneIfError(err)

		gp := newGenericProvider(runtimeCtx, svc.GetProvider())

		auth, isAuthPresent := payload.auth[provStr]

		if !isAuthPresent {
			authDTO, isAuthPresent := svc.GetProvider().GetAuth()
			if !isAuthPresent {
				printErrorAndExitOneIfError(fmt.Errorf("auth not present"))
			}
			auth = transformOpenapiStackqlAuthToLocal(authDTO)
		}

		err = runQueryCommand(
			gp,
			auth,
			payload,
		)

		printErrorAndExitOneIfError(err)

	},
}
