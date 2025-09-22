package argparse

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/pprof"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/client"
	"github.com/stackql/any-sdk/pkg/constants"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/pkg/internaldto"
	"github.com/stackql/any-sdk/pkg/local_template_executor"
	"github.com/stackql/any-sdk/pkg/stream_transform"
)

func getLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(os.Stderr)
	logger.SetLevel(logrus.WarnLevel)
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
	return anysdk.LoadProviderAndServiceFromPaths(qcp.provFilePath, qcp.svcFilePath)
}

func (qcp *queryCmdPayload) getProvider() (anysdk.Provider, error) {
	pb, err := os.ReadFile(qcp.provFilePath)
	if err != nil {
		return nil, err
	}
	prov, err := anysdk.LoadProviderDocFromBytes(pb)
	if err != nil {
		return nil, err
	}
	return prov, nil
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

func runQueryCommand(authCtx *dto.AuthCtx, payload *queryCmdPayload) error {
	prov, err := payload.getProvider()
	if err != nil {
		return err
	}
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
	protocolType, protocolTypeErr := prov.GetProtocolType()
	if protocolTypeErr != nil {
		return protocolTypeErr
	}
	switch protocolType {
	case client.LocalTemplated:
		inlines := opStore.GetInline()
		if len(inlines) == 0 {
			return fmt.Errorf("no inlines found")
		}
		executor := local_template_executor.NewLocalTemplateExecutor(
			inlines[0],
			inlines[1:],
			nil,
		)
		resp, err := executor.Execute(
			map[string]any{"parameters": payload.parameters},
		)
		if err != nil {
			return err
		}
		stdOut, stdOutExists := resp.GetStdOut()
		stdoutStr := ""
		if stdOutExists {
			stdoutStr = stdOut.String()
			expectedResponse, isExpectedResponse := opStore.GetResponse()
			if isExpectedResponse {
				responseTransform, responseTransformExists := expectedResponse.GetTransform()
				if responseTransformExists && responseTransform.GetType() == "golang_template_v0.1.0" {
					input := stdoutStr
					streamTransformerFactory := stream_transform.NewStreamTransformerFactory(
						responseTransform.GetType(),
						responseTransform.GetBody(),
					)
					if !streamTransformerFactory.IsTransformable() {
						return fmt.Errorf("unsupported template type: %s", responseTransform.GetType())
					}
					tfm, err := streamTransformerFactory.GetTransformer(input)
					if err != nil {
						return fmt.Errorf("failed to transform: %v", err)
					}
					transformError := tfm.Transform()
					if transformError != nil {
						return fmt.Errorf("failed to transform: %v", transformError)
					}
					outStream := tfm.GetOutStream()
					outBytes, err := io.ReadAll(outStream)
					if err != nil {
						return fmt.Errorf("failed to read out stream: %v", err)
					}
					outputStr := string(outBytes)
					stdoutStr = outputStr
				}
			}
			fmt.Fprintf(os.Stdout, "%s", stdoutStr)
		}
		stdErr, stdErrExists := resp.GetStdErr()
		if stdErrExists {
			fmt.Fprintf(os.Stderr, "%s", stdErr.String())
		}
		return nil
	case client.HTTP:
		var isFromAnnotation bool = false // TODO: publish something meaningful here
		prep := anysdk.NewHTTPPreparator(
			prov,
			svc,
			opStore,
			map[int]map[string]interface{}{
				0: payload.parameters,
			},
			nil,
			execCtx,
			getLogger(),
		)
		armoury, err := prep.BuildHTTPRequestCtx(anysdk.NewHTTPPreparatorConfig(isFromAnnotation))
		if err != nil {
			return err
		}
		for _, v := range armoury.GetRequestParams() {
			argList := v.GetArgList()

			cc := anysdk.NewAnySdkClientConfigurator(
				payload.rtCtx,
				prov.GetName(),
			)
			response, apiErr := anysdk.CallFromSignature(
				cc, payload.rtCtx, authCtx, authCtx.Type, false, os.Stderr, prov, anysdk.NewAnySdkOpStoreDesignation(opStore), argList)
			if apiErr != nil {
				return err
			}
			httpResponse, httpResponseErr := response.GetHttpResponse()
			if httpResponseErr != nil {
				return httpResponseErr
			}
			defer httpResponse.Body.Close()
			bodyBytes, err := io.ReadAll(httpResponse.Body)
			if err != nil {
				return err
			}
			expectedResponse, isExpectedResponse := opStore.GetResponse()
			if isExpectedResponse {
				responseTransform, responseTransformExists := expectedResponse.GetTransform()
				if responseTransformExists {
					streamTransformerFactory := stream_transform.NewStreamTransformerFactory(
						responseTransform.GetType(),
						responseTransform.GetBody(),
					)
					if !streamTransformerFactory.IsTransformable() {
						return fmt.Errorf("unsupported template type: %s", responseTransform.GetType())
					}
					tfm, err := streamTransformerFactory.GetTransformer(string(bodyBytes))
					if err != nil {
						return fmt.Errorf("template stream transform error: %v", err)
					}
					if err := tfm.Transform(); err != nil {
						return fmt.Errorf("failed to transform: %v", err)
					}
					outStream := tfm.GetOutStream()
					bodyBytes, err = io.ReadAll(outStream)
					if err != nil {
						return fmt.Errorf("failed to read out stream: %v", err)
					}
				}
			}
			fmt.Fprintf(os.Stdout, "%s", string(bodyBytes))
		}
		return nil
	default:
		return fmt.Errorf("protocol type = '%v' not supported", protocolType)
	}
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

		prov, err := payload.getProvider()

		printErrorAndExitOneIfError(err)

		provStr := prov.GetName()

		protocolType, protocolTypeErr := prov.GetProtocolType()

		printErrorAndExitOneIfError(protocolTypeErr)

		auth, isAuthPresent := payload.auth[provStr]

		if !isAuthPresent && protocolType == client.HTTP {
			authDTO, isAuthPresent := prov.GetAuth()
			if !isAuthPresent {
				printErrorAndExitOneIfError(fmt.Errorf("auth not present"))
			}
			auth = transformOpenapiStackqlAuthToLocal(authDTO)
		}

		err = runQueryCommand(
			auth,
			payload,
		)

		printErrorAndExitOneIfError(err)

	},
}
