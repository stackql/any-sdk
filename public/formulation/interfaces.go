// Code generated mechanically from wrappers.go (interfaces only) - DO NOT EDIT.
package formulation

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/lib/pq/oid"
	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/client"
	"github.com/stackql/any-sdk/pkg/constants"
	"github.com/stackql/any-sdk/pkg/httpelement"
	"github.com/stackql/any-sdk/pkg/internaldto"
	"github.com/stackql/any-sdk/pkg/netutils"
	"github.com/stackql/any-sdk/pkg/providerinvoker"
	"github.com/stackql/any-sdk/public/radix_tree_address_space"
	"github.com/stackql/stackql-parser/go/sqltypes"
	"github.com/stackql/stackql-provider-registry/signing/Ed25519/app/edcrypto"
)

// NOTE: Addressable is already defined as an interface in wrappers.go.

// AuthMetadata mirrors methods on *AuthMetadata
type AuthMetadata interface {
	GetHeaders() []string
	ToMap() map[string]interface{}
}

// AuthCtx mirrors methods on *AuthCtx
// type AuthCtx interface {
// 	Clone() *AuthCtx
// 	GetCredentialsBytes() ([]byte, error)
// 	GetCredentialsSourceDescriptorString() string
// 	GetSQLCfg() (SQLBackendCfg, bool)
// 	HasKey() bool
// }

// RuntimeCtx mirrors methods on RuntimeCtx
type RuntimeCtx interface {
	Copy() RuntimeCtx
}

// AddressSpace mirrors methods on AddressSpace
type AddressSpaceExpansionConfig interface {
	IsAsync() bool
	IsLegacy() bool
	IsAllowNilResponse() bool
	unwrap() anysdk.AddressSpaceExpansionConfig
}

// AddressSpaceGrammar defines the search DSL
type AddressSpaceGrammar radix_tree_address_space.AddressSpaceGrammar

type AddressSpace interface {
	GetGlobalSelectSchemas() map[string]Schema
	DereferenceAddress(address string) (any, bool)
	WriteToAddress(address string, val any) error
	ReadFromAddress(address string) (any, bool)
	ResolveSignature(map[string]any) (bool, map[string]any)
	Invoke(...any) error
	ToMap(AddressSpaceExpansionConfig) (map[string]any, error)
	ToRelation(AddressSpaceExpansionConfig) (Relation, error)
	unwrap() anysdk.AddressSpace
}

// Column mirrors methods on Column
type Column interface {
	GetName() string
	GetSchema() Schema
	GetWidth() int
}

// ExpectedRequest mirrors methods on ExpectedRequest
type ExpectedRequest interface {
	GetBodyMediaType() string
}

// ExpectedResponse mirrors methods on ExpectedResponse
type ExpectedResponse interface {
	GetObjectKey() string
	GetTransform() (Transform, bool)
}

// GraphQL mirrors methods on GraphQL
type GraphQL interface {
	GetCursorJSONPath() (string, bool)
	GetQuery() string
	GetResponseJSONPath() (string, bool)
	unwrap() anysdk.GraphQL
}

// HTTPArmoury mirrors methods on HTTPArmoury
type HTTPArmoury interface {
	GetRequestParams() []HTTPArmouryParameters
	SetRequestParams(p0 []HTTPArmouryParameters)
}

// HTTPArmouryParameters mirrors methods on HTTPArmouryParameters
type HTTPArmouryParameters interface {
	Encode() string
	GetArgList() client.AnySdkArgList
	GetParameters() HttpParameters
	GetQuery() url.Values
	GetRequest() *http.Request
	SetNextPage(ops OperationStore, token string, tokenKey HTTPElement) (*http.Request, error)
	SetRawQuery(p0 string)
	ToFlatMap() (map[string]interface{}, error)
}

// HTTPPreparator mirrors methods on HTTPPreparator
type HTTPPreparator interface {
	BuildHTTPRequestCtx(p0 anysdk.HTTPPreparatorConfig) (HTTPArmoury, error)
}

func newHTTPPreparatorFromAnySdkHTTPPreparator(inner anysdk.HTTPPreparator) HTTPPreparator {
	return &wrappedHTTPPreparator{
		inner: inner,
	}
}

// HttpParameters mirrors methods on HttpParameters
type HttpParameters interface {
	GetInlineParameterFlatMap() (map[string]interface{}, error)
	ToFlatMap() (map[string]interface{}, error)
}

// HttpPreparatorStream mirrors methods on HttpPreparatorStream
type HttpPreparatorStream interface {
	Next() (HTTPPreparator, bool)
	Write(p0 HTTPPreparator) error
}

// ITable mirrors methods on ITable
type ITable interface {
	GetKey(p0 string) (interface{}, error)
	GetKeyAsSqlVal(p0 string) (sqltypes.Value, error)
	GetName() string
	KeyExists(p0 string) bool
}

// MethodAnalysisOutput mirrors methods on MethodAnalysisOutput
type MethodAnalysisOutput interface {
	GetInsertTabulation() Tabulation
	GetItemSchema() (Schema, bool)
	GetOrderedStarColumnsNames() ([]string, error)
	GetSelectTabulation() Tabulation
	IsAwait() bool
	IsNilResponseAllowed() bool
}

// MethodAnalyzer mirrors methods on MethodAnalyzer
type MethodAnalyzer interface {
	AnalyzeUnaryAction(p0 anysdk.MethodAnalysisInput) (MethodAnalysisOutput, error)
}

// Methods mirrors methods on Methods
type Methods interface {
	OrderMethods() ([]StandardOperationStore, error)
}

// OperationInverse mirrors methods on OperationInverse
type OperationInverse interface {
	GetOperationStore() (StandardOperationStore, bool)
}

// OperationStore mirrors methods on OperationStore
type OperationStore interface {
	DeprecatedProcessResponse(response *http.Response) (map[string]interface{}, error)
	GetName() string
	GetNonBodyParameters() map[string]Addressable
	GetPaginationRequestTokenSemantic() (TokenSemantic, bool)
	GetPaginationResponseTokenSemantic() (TokenSemantic, bool)
	GetParameter(paramKey string) (Addressable, bool)
	GetRequestBodySchema() (Schema, error)
	GetRequiredNonBodyParameters() map[string]Addressable
	GetRequiredParameters() map[string]Addressable
	GetResource() Resource
	GetResponseBodySchemaAndMediaType() (Schema, string, error)
	GetSelectItemsKey() string
	GetService() OpenAPIService
	IsRequestBodyAttributeRenamed(p0 string) bool
	IsRequiredRequestBodyProperty(key string) bool
	ProcessResponse(p0 *http.Response) (ProcessedOperationResponse, error)
	RenameRequestBodyAttribute(p0 string) (string, error)
	RevertRequestBodyAttributeRename(p0 string) (string, error)
	GetProjections() map[string]string
	GetAddressSpace() (AddressSpace, bool)
	GetGraphQL() GraphQL
	unwrap() anysdk.OperationStore
}

// ProcessedOperationResponse mirrors methods on ProcessedOperationResponse
type ProcessedOperationResponse interface {
	GetResponse() (Response, bool)
	GetReversal() (HTTPPreparator, bool)
}

// Provider mirrors methods on Provider
type Provider interface {
	GetAuth() (AuthDTO, bool)
	GetDeleteItemsKey() string
	GetMinStackQLVersion() string
	GetName() string
	GetProtocolType() (client.ClientProtocolType, error)
	unwrap() anysdk.Provider
}

type ExecContext interface {
	GetExecPayload() internaldto.ExecPayload
	GetResource() Resource
	unwrap() anysdk.ExecContext
}

type RegistryConfig struct {
	RegistryURL      string                   `json:"url" yaml:"url"`
	SrcPrefix        *string                  `json:"srcPrefix" yaml:"srcPrefix"`
	DistPrefix       *string                  `json:"distPrefix" yaml:"distPrefix"`
	AllowSrcDownload bool                     `json:"allowSrcDownload" yaml:"allowSrcDownload"`
	LocalDocRoot     string                   `json:"localDocRoot" yaml:"localDocRoot"`
	VerifyConfig     *edcrypto.VerifierConfig `json:"verifyConfig" yaml:"verifyConfig"`
}

func (rc RegistryConfig) toAnySdkRegistryConfig() anysdk.RegistryConfig {
	return anysdk.RegistryConfig{
		RegistryURL:      rc.RegistryURL,
		SrcPrefix:        rc.SrcPrefix,
		DistPrefix:       rc.DistPrefix,
		AllowSrcDownload: rc.AllowSrcDownload,
		LocalDocRoot:     rc.LocalDocRoot,
		VerifyConfig:     rc.VerifyConfig,
	}
}

// ProviderDescription mirrors methods on ProviderDescription
type ProviderDescription interface {
	GetLatestVersion() (string, error)
	Versions() []string
}

// ProviderService mirrors methods on ProviderService
type ProviderService interface {
	GetDescription() string
	GetID() string
	GetName() string
	GetTitle() string
	GetVersion() string
	IsPreferred() bool
}

type MethodSet interface {
	GetFirstMatch(params map[string]interface{}) (StandardOperationStore, map[string]interface{}, bool)
	GetFirstNamespaceMatch(params map[string]any) (StandardOperationStore, map[string]any, bool)
	GetFirst() (StandardOperationStore, string, bool)
	Size() int
}

// RegistryAPI mirrors methods on RegistryAPI
type RegistryAPI interface {
	ClearProviderCache(p0 string) error
	GetLatestPublishedVersion(p0 string) (string, error)
	ListAllAvailableProviders() (map[string]ProviderDescription, error)
	ListAllProviderVersions(p0 string) (map[string]ProviderDescription, error)
	ListLocallyAvailableProviders() map[string]ProviderDescription
	LoadProviderByName(p0 string, p1 string) (Provider, error)
	PullAndPersistProviderArchive(p0 string, p1 string) error
	RemoveProviderVersion(p0 string, p1 string) error
}

// Relation mirrors methods on Relation
type Relation interface {
	GetColumnDescriptors() []ColumnDescriptor
	GetColumns() []Column
}

// Resource mirrors methods on Resource
type Resource interface {
	FindMethod(key string) (StandardOperationStore, error)
	GetFirstMethodFromSQLVerb(sqlVerb string) (StandardOperationStore, string, bool)
	GetFirstNamespaceMethodMatchFromSQLVerb(sqlVerb string, parameters map[string]interface{}) (StandardOperationStore, map[string]interface{}, bool)
	GetID() string
	GetMethodsMatched() Methods
	GetName() string
	GetViewsForSqlDialect(sqlDialect string) ([]View, bool)
	ToMap(extended bool) map[string]interface{}
	unwrap() anysdk.Resource
}

// SQLExternalColumn mirrors methods on SQLExternalColumn
type SQLExternalColumn interface {
	GetName() string
	GetOid() uint32
	GetPrecision() int
	GetType() string
	GetWidth() int
}

// SQLExternalTable mirrors methods on SQLExternalTable
type SQLExternalTable interface {
	GetCatalogName() string
	GetColumns() []SQLExternalColumn
	GetName() string
	GetSchemaName() string
	unwrap() anysdk.SQLExternalTable
}

type OpenAPIService interface {
	Service
	unwrapOpenapi3Service() anysdk.OpenAPIService
}

// Schema mirrors methods on Schema
type Schema interface {
	FindByPath(path string, visited map[string]bool) Schema
	GetAdditionalProperties() (Schema, bool)
	GetAllColumns(p0 string) []string
	GetItemsSchema() (Schema, error)
	GetName() string
	GetProperties() (Schemas, error)
	GetProperty(propertyKey string) (Schema, bool)
	GetPropertySchema(key string) (Schema, error)
	GetSelectSchema(itemsKey string, mediaType string) (Schema, string, error)
	GetSelectionName() string
	GetTitle() string
	GetType() string
	IsBoolean() bool
	IsFloat() bool
	IsIntegral() bool
	IsReadOnly() bool
	IsRequired(key string) bool
	SetKey(p0 string)
	Tabulate(p0 bool, p1 string) Tabulation
	ToDescriptionMap(extended bool) map[string]interface{}
	unwrap() anysdk.Schema
}

type Schemas map[string]Schema

// Service mirrors methods on Service
type Service interface {
	GetResource(resourceName string) (Resource, error)
	GetSchema(key string) (Schema, error)
	GetServers() (openapi3.Servers, bool)
	unwrap() anysdk.Service
}

// StandardOperationStore mirrors methods on StandardOperationStore
type StandardOperationStore interface {
	OperationStore
	GetServers() (openapi3.Servers, bool)
	unwrapStandardOperationStore() anysdk.StandardOperationStore
}

// Tabulation mirrors methods on Tabulation
type Tabulation interface {
	GetColumns() []ColumnDescriptor
	PushBackColumn(col ColumnDescriptor)
	RenameColumnsToXml() Tabulation
}

type TokenTransformer func(interface{}) (interface{}, error)

// TokenSemantic mirrors methods on TokenSemantic
type TokenSemantic interface {
	GetKey() string
	GetLocation() string
	GetTransformer() (TokenTransformer, error)
}

// Transform mirrors methods on Transform
type Transform interface {
	GetBody() string
	GetType() string
}

// View mirrors methods on View
type View interface {
	GetDDL() string
	GetNameNaive() string
	GetRequiredParamNames() []string
	unwrap() anysdk.View
}

// AuthUtility mirrors methods on AuthUtility
type AuthUtility interface {
	ActivateAuth(authCtx *AuthCtx, principal string, authType string)
	ApiTokenAuth(authCtx *AuthCtx, httpContext netutils.HTTPContext, enforceBearer bool) (*http.Client, error)
	AuthRevoke(authCtx *AuthCtx) error
	AwsSigningAuth(authCtx *AuthCtx, httpContext netutils.HTTPContext) (*http.Client, error)
	AzureDefaultAuth(authCtx *AuthCtx, httpContext netutils.HTTPContext) (*http.Client, error)
	BasicAuth(authCtx *AuthCtx, httpContext netutils.HTTPContext) (*http.Client, error)
	CustomAuth(authCtx *AuthCtx, httpContext netutils.HTTPContext) (*http.Client, error)
	GCloudOAuth(runtimeCtx RuntimeCtx, authCtx *AuthCtx, enforceRevokeFirst bool) (*http.Client, error)
	GenericOauthClientCredentials(authCtx *AuthCtx, scopes []string, httpContext netutils.HTTPContext) (*http.Client, error)
	GetCurrentGCloudOauthUser() ([]byte, error)
	GoogleOauthServiceAccount(provider string, authCtx *AuthCtx, scopes []string, httpContext netutils.HTTPContext) (*http.Client, error)
	ParseServiceAccountFile(ac *AuthCtx) (any, error)
}

// AnySdkClientConfigurator mirrors methods on AnySdkClientConfigurator
type AnySdkClientConfigurator interface {
	Auth(authCtx *AuthCtx, authTypeRequested string, enforceRevokeFirst bool) (client.AnySdkClient, error)
}

// AnySdkResponse mirrors methods on AnySdkResponse
type AnySdkResponse interface {
	GetHttpResponse() (*http.Response, error)
}

// ControlAttributes mirrors methods on ControlAttributes
type ControlAttributes interface {
	GetControlGCStatusColumnName() string
	GetControlGenIDColumnName() string
	GetControlInsIDColumnName() string
	GetControlInsertEncodedIDColumnName() string
	GetControlLatestUpdateColumnName() string
	GetControlMaxTxnColumnName() string
	GetControlSsnIDColumnName() string
	GetControlTxnIDColumnName() string
}

// AuthContexts mirrors methods on AuthContexts
type AuthContexts interface {
	Clone() AuthContexts
}

// DataFlowCfg mirrors methods on DataFlowCfg
type DataFlowCfg interface {
	GetMaxDependencies() int
}

// NamespaceCfg mirrors methods on NamespaceCfg
type NamespaceCfg interface {
	GetRegex() (*regexp.Regexp, error)
	GetTemplate() (*template.Template, error)
}

// OutputPacket mirrors methods on OutputPacket
type OutputPacket interface {
	GetColumnNames() []string
	GetColumnOIDs() []oid.Oid
	GetRawRows() map[int]map[int]interface{}
	GetRows() map[string]map[string]interface{}
}

// PgTLSCfg mirrors methods on PgTLSCfg
type PgTLSCfg interface {
	GetKeyPair() (tls.Certificate, error)
}

// SQLBackendCfg mirrors methods on SQLBackendCfg
type SQLBackendCfg interface {
	GetDatabaseName() (string, error)
	GetIntelViewSchemaName() string
	GetOpsViewSchemaName() string
	GetSQLDialect() string
	GetSchemaType() string
	GetTableSchemaName() string
}

// SessionContext mirrors methods on SessionContext
type SessionContext interface {
	GetIsolationLevel() constants.IsolationLevel
	GetRollbackType() constants.RollbackType
	UpdateIsolationLevel(p0 string) error
}

// TxnCoordinatorCfg mirrors methods on TxnCoordinatorCfg
type TxnCoordinatorCfg interface {
	GetMaxTxnDepth() int
}

// GQLReader mirrors methods on GQLReader
type GQLReader interface {
	Read() ([]map[string]interface{}, error)
}

// ExecPayload mirrors methods on ExecPayload
type ExecPayload interface {
	GetPayloadMap() map[string]interface{}
}

// HTTPElement mirrors methods on HTTPElement (internaldto-backed)
type HTTPElement interface {
	GetName() string
	GetType() internaldto.HTTPElementType
	SetTransformer(transformer func(interface{}) (interface{}, error))
	IsTransformerPresent() bool
	Transformer(t interface{}) (interface{}, error)
}

// ExecutionResponse mirrors methods on ExecutionResponse
type ExecutionResponse interface {
	GetStdErr() (*bytes.Buffer, bool)
	GetStdOut() (*bytes.Buffer, bool)
}

// Executor mirrors methods on Executor
type Executor interface {
	Execute(p0 map[string]any) (ExecutionResponse, error)
}

// NameMangler mirrors methods on NameMangler
type NameMangler interface {
	MangleName(p0 string, p1 ...any) string
}

// ActionInsertPayload mirrors methods on ActionInsertPayload
type ActionInsertPayload interface {
	GetItemisationResult() ItemisationResult
	GetParamsUsed() map[string]interface{}
	GetReqEncoding() string
	GetTableName() string
	IsHousekeepingDone() bool
}

// ActionInsertResult mirrors methods on ActionInsertResult
type ActionInsertResult interface {
	GetError() (error, bool)
	IsHousekeepingDone() bool
}

// InsertPreparator mirrors methods on InsertPreparator
type InsertPreparator interface {
	ActionInsertPreparation(payload ActionInsertPayload) ActionInsertResult
}

// Invoker mirrors methods on Invoker
type Invoker interface {
	Invoke(ctx context.Context, req providerinvoker.Request) (providerinvoker.Result, error)
}

// ItemisationResult mirrors methods on ItemisationResult
type ItemisationResult interface {
	GetItems() (interface{}, bool)
}

// Response mirrors methods on Response
type Response interface {
	Error() string
	ExtractElement(e HTTPHTTPElement) (interface{}, error)
	GetHttpResponse() *http.Response
	GetProcessedBody() interface{}
	HasError() bool
}

// HTTPHTTPElement mirrors methods on HTTPHTTPElement (httpelement-backed)
type HTTPHTTPElement interface {
	GetName() string
	GetLocation() httpelement.HTTPElementLocation
}

// StreamTransformer mirrors methods on StreamTransformer
type StreamTransformer interface {
	GetOutStream() io.Reader
	Transform() error
}

// StreamTransformerFactory mirrors methods on StreamTransformerFactory
type StreamTransformerFactory interface {
	GetTransformer(input string) (StreamTransformer, error)
	IsTransformable() bool
}

// MapReader mirrors methods on MapReader
type MapReader interface {
	Read() ([]map[string]interface{}, error)
}

// MapStream mirrors methods on MapStream
type MapStream interface {
	Write(p0 []map[string]interface{}) error
}

// MapStreamCollection mirrors methods on MapStreamCollection
type MapStreamCollection interface {
	Len() int
	Push(p0 MapStream)
}

// AuthDTO mirrors methods on AuthDTO
type AuthDTO interface {
	GetAccountID() string
	GetAccountIDEnvVar() string
	GetAuthStyle() int
	GetClientID() string
	GetClientIDEnvVar() string
	GetClientSecret() string
	GetClientSecretEnvVar() string
	GetEnvVarAPIKeyStr() string
	GetEnvVarAPISecretStr() string
	GetEnvVarPassword() string
	GetEnvVarUsername() string
	GetGrantType() string
	GetInlineBasicCredentials() string
	GetKeyEnvVar() string
	GetKeyFilePath() string
	GetKeyFilePathEnvVar() string
	GetKeyID() string
	GetKeyIDEnvVar() string
	GetLocation() string
	GetName() string
	GetScopes() []string
	GetSubject() string
	GetSuccessor() (AuthDTO, bool)
	GetTokenURL() string
	GetType() string
	GetValuePrefix() string
	GetValues() url.Values
}

// IDiscoveryAdapter mirrors methods on IDiscoveryAdapter
type IDiscoveryAdapter interface {
	GetProvider(providerKey string) (Provider, error)
	GetResourcesMap(prov Provider, serviceKey string) (map[string]Resource, error)
	GetServiceHandlesMap(prov Provider) (map[string]ProviderService, error)
	GetServiceShard(prov Provider, serviceKey string, resourceKey string) (Service, error)
	PersistStaticExternalSQLDataSource(prov Provider) error
}

// AddressSpaceFormulator mirrors methods on AddressSpaceFormulator
type AddressSpaceFormulator interface {
	Formulate() error
	GetAddressSpace() AddressSpace
}

// SQLEngine mirrors methods on SQLEngine
type SQLEngine interface {
	CacheStoreGet(p0 string) ([]byte, error)
	CacheStorePut(p0 string, p1 []byte, p2 string, p3 int) error
	Exec(p0 string, p1 ...interface{}) (sql.Result, error)
	ExecInTxn(queries []string) error
	GetCurrentDiscoveryGenerationID(discoveryID string) (int, error)
	GetCurrentGenerationID() (int, error)
	GetDB() (*sql.DB, error)
	GetNextDiscoveryGenerationID(discoveryID string) (int, error)
	GetNextGenerationID() (int, error)
	GetNextSessionID(p0 int) (int, error)
	GetTx() (*sql.Tx, error)
	Query(p0 string, p1 ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}
