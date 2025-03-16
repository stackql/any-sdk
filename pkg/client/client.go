package client

import (
	"fmt"
	"net/http"

	"github.com/stackql/any-sdk/pkg/dto"
)

/*

We model arbiitrary remote and local interfaces like C functions, composed of:

- Name.
- Argument list.
- Return type.

In the fist instance, support for stateful TCP/IP protocols, unix sockets and standard os primitives for spawned process communication are targeted.

Aspirationally, socketless (network) protocols, sundry inter-process communication mechanisms are further targets.

*/

type ClientProtocolType int

const (
	ClientProtocolTypeHTTP           string = "http"
	ClientProtocolTypeLocalTemplated string = "local_templated"
)

const (
	HTTP ClientProtocolType = iota
	LocalTemplated
	Disallowed
)

func ClientProtocolTypeFromString(s string) (ClientProtocolType, error) {
	switch s {
	case ClientProtocolTypeHTTP:
		return HTTP, nil
	case ClientProtocolTypeLocalTemplated:
		return LocalTemplated, nil
	default:
		return Disallowed, fmt.Errorf("unsupported protocol type: %s", s)
	}
}

type AnySdkResponse interface {
	IsErroneous() bool
	GetHttpResponse() (*http.Response, error)
}

type AnySdkDesignation interface {
	GetDesignation() (interface{}, bool)
}

type AnySdkArg interface {
	GetArg() (interface{}, bool)
}

type AnySdkArgList interface {
	GetArgs() []AnySdkArg
	GetProtocolType() ClientProtocolType
}

type AnySdkInvocation interface {
	GetDesignation() (AnySdkDesignation, bool)
	GetArgs() (AnySdkArgList, bool)
}

type AnySdkClient interface {
	Do(AnySdkDesignation, AnySdkArgList) (AnySdkResponse, error)
}

type AnySdkClientConfigurator interface {
	Auth(
		authCtx *dto.AuthCtx,
		authTypeRequested string,
		enforceRevokeFirst bool,
	) (AnySdkClient, error)
	InferAuthType(authCtx dto.AuthCtx, authTypeRequested string) string
}

type ClientConfiguratorInput interface {
	GetAuthContext() *dto.AuthCtx
	GetAuthType() string
	GetEnforceRevokeFirst() bool
}
