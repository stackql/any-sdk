# StackQL live any-sdk receiver method usage

Generated from `surface_usage.txt` (typed selection analysis). Each section below is a **thin interface** you can define in `pkg/surface` and then implement via wrappers in `public/formulation`.

## Exhaustive thin interface list

### AuthMetadata

- receiver: `*github.com/stackql/any-sdk/anysdk.AuthMetadata`

- methods:

  - `GetHeaders func() []string`
  - `ToMap func() map[string]interface{}`

### AuthCtx

- receiver: `*github.com/stackql/any-sdk/pkg/dto.AuthCtx`

- methods:

  - `Clone func() *github.com/stackql/any-sdk/pkg/dto.AuthCtx`
  - `GetCredentialsBytes func() ([]byte, error)`
  - `GetCredentialsSourceDescriptorString func() string`
  - `GetSQLCfg func() (github.com/stackql/any-sdk/pkg/dto.SQLBackendCfg, bool)`
  - `HasKey func() bool`

### RuntimeCtx

- receiver: `*github.com/stackql/any-sdk/pkg/dto.RuntimeCtx`

- methods:

  - `Set func(key string, val string) error`

### AddressSpace

- receiver: `github.com/stackql/any-sdk/anysdk.AddressSpace`

- methods:

  - `ToRelation func(github.com/stackql/any-sdk/anysdk.AddressSpaceExpansionConfig) (github.com/stackql/any-sdk/anysdk.Relation, error)`

### Addressable

- receiver: `github.com/stackql/any-sdk/anysdk.Addressable`

- methods:

  - `ConditionIsValid func(lhs string, rhs interface{}) bool`
  - `GetName func() string`
  - `GetType func() string`

### Column

- receiver: `github.com/stackql/any-sdk/anysdk.Column`

- methods:

  - `GetName func() string`
  - `GetSchema func() github.com/stackql/any-sdk/anysdk.Schema`
  - `GetWidth func() int`

### ExpectedRequest

- receiver: `github.com/stackql/any-sdk/anysdk.ExpectedRequest`

- methods:

  - `GetBodyMediaType func() string`

### ExpectedResponse

- receiver: `github.com/stackql/any-sdk/anysdk.ExpectedResponse`

- methods:

  - `GetObjectKey func() string`
  - `GetTransform func() (github.com/stackql/any-sdk/anysdk.Transform, bool)`

### GraphQL

- receiver: `github.com/stackql/any-sdk/anysdk.GraphQL`

- methods:

  - `GetCursorJSONPath func() (string, bool)`
  - `GetQuery func() string`
  - `GetResponseJSONPath func() (string, bool)`

### HTTPArmoury

- receiver: `github.com/stackql/any-sdk/anysdk.HTTPArmoury`

- methods:

  - `GetRequestParams func() []github.com/stackql/any-sdk/anysdk.HTTPArmouryParameters`
  - `SetRequestParams func([]github.com/stackql/any-sdk/anysdk.HTTPArmouryParameters)`

### HTTPArmouryParameters

- receiver: `github.com/stackql/any-sdk/anysdk.HTTPArmouryParameters`

- methods:

  - `Encode func() string`
  - `GetArgList func() github.com/stackql/any-sdk/pkg/client.AnySdkArgList`
  - `GetParameters func() github.com/stackql/any-sdk/anysdk.HttpParameters`
  - `GetQuery func() net/url.Values`
  - `GetRequest func() *net/http.Request`
  - `SetNextPage func(ops github.com/stackql/any-sdk/anysdk.OperationStore, token string, tokenKey github.com/stackql/any-sdk/pkg/internaldto.HTTPElement) (*net/http.Request, error)`
  - `SetRawQuery func(string)`
  - `ToFlatMap func() (map[string]interface{}, error)`

### HTTPPreparator

- receiver: `github.com/stackql/any-sdk/anysdk.HTTPPreparator`

- methods:

  - `BuildHTTPRequestCtx func(github.com/stackql/any-sdk/anysdk.HTTPPreparatorConfig) (github.com/stackql/any-sdk/anysdk.HTTPArmoury, error)`

### HttpParameters

- receiver: `github.com/stackql/any-sdk/anysdk.HttpParameters`

- methods:

  - `GetInlineParameterFlatMap func() (map[string]interface{}, error)`
  - `ToFlatMap func() (map[string]interface{}, error)`

### HttpPreparatorStream

- receiver: `github.com/stackql/any-sdk/anysdk.HttpPreparatorStream`

- methods:

  - `Next func() (github.com/stackql/any-sdk/anysdk.HTTPPreparator, bool)`
  - `Write func(github.com/stackql/any-sdk/anysdk.HTTPPreparator) error`

### ITable

- receiver: `github.com/stackql/any-sdk/anysdk.ITable`

- methods:

  - `GetKey func(string) (interface{}, error)`
  - `GetKeyAsSqlVal func(string) (github.com/stackql/stackql-parser/go/sqltypes.Value, error)`
  - `GetName func() string`
  - `KeyExists func(string) bool`

### MethodAnalysisOutput

- receiver: `github.com/stackql/any-sdk/anysdk.MethodAnalysisOutput`

- methods:

  - `GetInsertTabulation func() github.com/stackql/any-sdk/anysdk.Tabulation`
  - `GetItemSchema func() (github.com/stackql/any-sdk/anysdk.Schema, bool)`
  - `GetOrderedStarColumnsNames func() ([]string, error)`
  - `GetSelectTabulation func() github.com/stackql/any-sdk/anysdk.Tabulation`
  - `IsAwait func() bool`
  - `IsNilResponseAllowed func() bool`

### MethodAnalyzer

- receiver: `github.com/stackql/any-sdk/anysdk.MethodAnalyzer`

- methods:

  - `AnalyzeUnaryAction func(github.com/stackql/any-sdk/anysdk.MethodAnalysisInput) (github.com/stackql/any-sdk/anysdk.MethodAnalysisOutput, error)`

### Methods

- receiver: `github.com/stackql/any-sdk/anysdk.Methods`

- methods:

  - `OrderMethods func() ([]github.com/stackql/any-sdk/anysdk.StandardOperationStore, error)`

### OperationInverse

- receiver: `github.com/stackql/any-sdk/anysdk.OperationInverse`

- methods:

  - `GetOperationStore func() (github.com/stackql/any-sdk/anysdk.StandardOperationStore, bool)`

### OperationStore

- receiver: `github.com/stackql/any-sdk/anysdk.OperationStore`

- methods:

  - `DeprecatedProcessResponse func(response *net/http.Response) (map[string]interface{}, error)`
  - `GetName func() string`
  - `GetNonBodyParameters func() map[string]github.com/stackql/any-sdk/anysdk.Addressable`
  - `GetPaginationRequestTokenSemantic func() (github.com/stackql/any-sdk/anysdk.TokenSemantic, bool)`
  - `GetPaginationResponseTokenSemantic func() (github.com/stackql/any-sdk/anysdk.TokenSemantic, bool)`
  - `GetParameter func(paramKey string) (github.com/stackql/any-sdk/anysdk.Addressable, bool)`
  - `GetRequestBodySchema func() (github.com/stackql/any-sdk/anysdk.Schema, error)`
  - `GetRequiredNonBodyParameters func() map[string]github.com/stackql/any-sdk/anysdk.Addressable`
  - `GetRequiredParameters func() map[string]github.com/stackql/any-sdk/anysdk.Addressable`
  - `GetResource func() github.com/stackql/any-sdk/anysdk.Resource`
  - `GetResponseBodySchemaAndMediaType func() (github.com/stackql/any-sdk/anysdk.Schema, string, error)`
  - `GetSelectItemsKey func() string`
  - `GetService func() github.com/stackql/any-sdk/anysdk.OpenAPIService`
  - `IsRequestBodyAttributeRenamed func(string) bool`
  - `IsRequiredRequestBodyProperty func(key string) bool`
  - `ProcessResponse func(*net/http.Response) (github.com/stackql/any-sdk/anysdk.ProcessedOperationResponse, error)`
  - `RenameRequestBodyAttribute func(string) (string, error)`
  - `RevertRequestBodyAttributeRename func(string) (string, error)`

### ProcessedOperationResponse

- receiver: `github.com/stackql/any-sdk/anysdk.ProcessedOperationResponse`

- methods:

  - `GetResponse func() (github.com/stackql/any-sdk/pkg/response.Response, bool)`
  - `GetReversal func() (github.com/stackql/any-sdk/anysdk.HTTPPreparator, bool)`

### Provider

- receiver: `github.com/stackql/any-sdk/anysdk.Provider`

- methods:

  - `GetAuth func() (github.com/stackql/any-sdk/pkg/surface.AuthDTO, bool)`
  - `GetDeleteItemsKey func() string`
  - `GetMinStackQLVersion func() string`
  - `GetName func() string`
  - `GetProtocolType func() (github.com/stackql/any-sdk/pkg/client.ClientProtocolType, error)`

### ProviderDescription

- receiver: `github.com/stackql/any-sdk/anysdk.ProviderDescription`

- methods:

  - `GetLatestVersion func() (string, error)`

### ProviderService

- receiver: `github.com/stackql/any-sdk/anysdk.ProviderService`

- methods:

  - `GetDescription func() string`
  - `GetID func() string`
  - `GetName func() string`
  - `GetTitle func() string`
  - `GetVersion func() string`
  - `IsPreferred func() bool`

### RegistryAPI

- receiver: `github.com/stackql/any-sdk/anysdk.RegistryAPI`

- methods:

  - `ClearProviderCache func(string) error`
  - `GetLatestPublishedVersion func(string) (string, error)`
  - `ListAllAvailableProviders func() (map[string]github.com/stackql/any-sdk/anysdk.ProviderDescription, error)`
  - `ListAllProviderVersions func(string) (map[string]github.com/stackql/any-sdk/anysdk.ProviderDescription, error)`
  - `ListLocallyAvailableProviders func() map[string]github.com/stackql/any-sdk/anysdk.ProviderDescription`
  - `LoadProviderByName func(string, string) (github.com/stackql/any-sdk/anysdk.Provider, error)`
  - `PullAndPersistProviderArchive func(string, string) error`
  - `RemoveProviderVersion func(string, string) error`

### Relation

- receiver: `github.com/stackql/any-sdk/anysdk.Relation`

- methods:

  - `GetColumnDescriptors func() []github.com/stackql/any-sdk/anysdk.ColumnDescriptor`
  - `GetColumns func() []github.com/stackql/any-sdk/anysdk.Column`

### Resource

- receiver: `github.com/stackql/any-sdk/anysdk.Resource`

- methods:

  - `FindMethod func(key string) (github.com/stackql/any-sdk/anysdk.StandardOperationStore, error)`
  - `GetFirstMethodFromSQLVerb func(sqlVerb string) (github.com/stackql/any-sdk/anysdk.StandardOperationStore, string, bool)`
  - `GetFirstNamespaceMethodMatchFromSQLVerb func(sqlVerb string, parameters map[string]interface{}) (github.com/stackql/any-sdk/anysdk.StandardOperationStore, map[string]interface{}, bool)`
  - `GetID func() string`
  - `GetMethodsMatched func() github.com/stackql/any-sdk/anysdk.Methods`
  - `GetName func() string`
  - `GetViewsForSqlDialect func(sqlDialect string) ([]github.com/stackql/any-sdk/anysdk.View, bool)`
  - `ToMap func(extended bool) map[string]interface{}`

### SQLExternalColumn

- receiver: `github.com/stackql/any-sdk/anysdk.SQLExternalColumn`

- methods:

  - `GetName func() string`
  - `GetOid func() uint32`
  - `GetPrecision func() int`
  - `GetType func() string`
  - `GetWidth func() int`

### SQLExternalTable

- receiver: `github.com/stackql/any-sdk/anysdk.SQLExternalTable`

- methods:

  - `GetCatalogName func() string`
  - `GetColumns func() []github.com/stackql/any-sdk/anysdk.SQLExternalColumn`
  - `GetName func() string`
  - `GetSchemaName func() string`

### Schema

- receiver: `github.com/stackql/any-sdk/anysdk.Schema`

- methods:

  - `FindByPath func(path string, visited map[string]bool) github.com/stackql/any-sdk/anysdk.Schema`
  - `GetAdditionalProperties func() (github.com/stackql/any-sdk/anysdk.Schema, bool)`
  - `GetAllColumns func(string) []string`
  - `GetItemsSchema func() (github.com/stackql/any-sdk/anysdk.Schema, error)`
  - `GetName func() string`
  - `GetProperties func() (github.com/stackql/any-sdk/anysdk.Schemas, error)`
  - `GetProperty func(propertyKey string) (github.com/stackql/any-sdk/anysdk.Schema, bool)`
  - `GetPropertySchema func(key string) (github.com/stackql/any-sdk/anysdk.Schema, error)`
  - `GetSelectSchema func(itemsKey string, mediaType string) (github.com/stackql/any-sdk/anysdk.Schema, string, error)`
  - `GetSelectionName func() string`
  - `GetTitle func() string`
  - `GetType func() string`
  - `IsBoolean func() bool`
  - `IsFloat func() bool`
  - `IsIntegral func() bool`
  - `IsReadOnly func() bool`
  - `IsRequired func(key string) bool`
  - `SetKey func(string)`
  - `Tabulate func(bool, string) github.com/stackql/any-sdk/anysdk.Tabulation`
  - `ToDescriptionMap func(extended bool) map[string]interface{}`

### Service

- receiver: `github.com/stackql/any-sdk/anysdk.Service`

- methods:

  - `GetResource func(resourceName string) (github.com/stackql/any-sdk/anysdk.Resource, error)`
  - `GetSchema func(key string) (github.com/stackql/any-sdk/anysdk.Schema, error)`
  - `GetServers func() (github.com/getkin/kin-openapi/openapi3.Servers, bool)`

### StandardOperationStore

- receiver: `github.com/stackql/any-sdk/anysdk.StandardOperationStore`

- methods:

  - `GetAddressSpace func() (github.com/stackql/any-sdk/anysdk.AddressSpace, bool)`
  - `GetColumnOrder func(extended bool) []string`
  - `GetGraphQL func() github.com/stackql/any-sdk/anysdk.GraphQL`
  - `GetInline func() []string`
  - `GetInverse func() (github.com/stackql/any-sdk/anysdk.OperationInverse, bool)`
  - `GetName func() string`
  - `GetOptionalParameters func() map[string]github.com/stackql/any-sdk/anysdk.Addressable`
  - `GetPaginationRequestTokenSemantic func() (github.com/stackql/any-sdk/anysdk.TokenSemantic, bool)`
  - `GetPaginationResponseTokenSemantic func() (github.com/stackql/any-sdk/anysdk.TokenSemantic, bool)`
  - `GetParameter func(paramKey string) (github.com/stackql/any-sdk/anysdk.Addressable, bool)`
  - `GetProjections func() map[string]string`
  - `GetRequest func() (github.com/stackql/any-sdk/anysdk.ExpectedRequest, bool)`
  - `GetRequestBodySchema func() (github.com/stackql/any-sdk/anysdk.Schema, error)`
  - `GetRequiredParameters func() map[string]github.com/stackql/any-sdk/anysdk.Addressable`
  - `GetResponse func() (github.com/stackql/any-sdk/anysdk.ExpectedResponse, bool)`
  - `GetResponseBodySchemaAndMediaType func() (github.com/stackql/any-sdk/anysdk.Schema, string, error)`
  - `GetSelectItemsKey func() string`
  - `GetSelectSchemaAndObjectPath func() (github.com/stackql/any-sdk/anysdk.Schema, string, error)`
  - `GetServers func() (github.com/getkin/kin-openapi/openapi3.Servers, bool)`
  - `IsAwaitable func() bool`
  - `IsNullary func() bool`
  - `ToPresentationMap func(extended bool) map[string]interface{}`

### Tabulation

- receiver: `github.com/stackql/any-sdk/anysdk.Tabulation`

- methods:

  - `GetColumns func() []github.com/stackql/any-sdk/anysdk.ColumnDescriptor`
  - `PushBackColumn func(col github.com/stackql/any-sdk/anysdk.ColumnDescriptor)`
  - `RenameColumnsToXml func() github.com/stackql/any-sdk/anysdk.Tabulation`

### TokenSemantic

- receiver: `github.com/stackql/any-sdk/anysdk.TokenSemantic`

- methods:

  - `GetKey func() string`
  - `GetLocation func() string`
  - `GetTransformer func() (github.com/stackql/any-sdk/anysdk.TokenTransformer, error)`

### Transform

- receiver: `github.com/stackql/any-sdk/anysdk.Transform`

- methods:

  - `GetBody func() string`
  - `GetType func() string`

### View

- receiver: `github.com/stackql/any-sdk/anysdk.View`

- methods:

  - `GetDDL func() string`
  - `GetNameNaive func() string`
  - `GetRequiredParamNames func() []string`

### AuthUtility

- receiver: `github.com/stackql/any-sdk/pkg/auth_util.AuthUtility`

- methods:

  - `ActivateAuth func(authCtx *github.com/stackql/any-sdk/pkg/dto.AuthCtx, principal string, authType string)`
  - `ApiTokenAuth func(authCtx *github.com/stackql/any-sdk/pkg/dto.AuthCtx, httpContext github.com/stackql/any-sdk/pkg/netutils.HTTPContext, enforceBearer bool) (*net/http.Client, error)`
  - `AuthRevoke func(authCtx *github.com/stackql/any-sdk/pkg/dto.AuthCtx) error`
  - `AwsSigningAuth func(authCtx *github.com/stackql/any-sdk/pkg/dto.AuthCtx, httpContext github.com/stackql/any-sdk/pkg/netutils.HTTPContext) (*net/http.Client, error)`
  - `AzureDefaultAuth func(authCtx *github.com/stackql/any-sdk/pkg/dto.AuthCtx, httpContext github.com/stackql/any-sdk/pkg/netutils.HTTPContext) (*net/http.Client, error)`
  - `BasicAuth func(authCtx *github.com/stackql/any-sdk/pkg/dto.AuthCtx, httpContext github.com/stackql/any-sdk/pkg/netutils.HTTPContext) (*net/http.Client, error)`
  - `CustomAuth func(authCtx *github.com/stackql/any-sdk/pkg/dto.AuthCtx, httpContext github.com/stackql/any-sdk/pkg/netutils.HTTPContext) (*net/http.Client, error)`
  - `GCloudOAuth func(runtimeCtx github.com/stackql/any-sdk/pkg/dto.RuntimeCtx, authCtx *github.com/stackql/any-sdk/pkg/dto.AuthCtx, enforceRevokeFirst bool) (*net/http.Client, error)`
  - `GenericOauthClientCredentials func(authCtx *github.com/stackql/any-sdk/pkg/dto.AuthCtx, scopes []string, httpContext github.com/stackql/any-sdk/pkg/netutils.HTTPContext) (*net/http.Client, error)`
  - `GetCurrentGCloudOauthUser func() ([]byte, error)`
  - `GoogleOauthServiceAccount func(provider string, authCtx *github.com/stackql/any-sdk/pkg/dto.AuthCtx, scopes []string, httpContext github.com/stackql/any-sdk/pkg/netutils.HTTPContext) (*net/http.Client, error)`
  - `ParseServiceAccountFile func(ac *github.com/stackql/any-sdk/pkg/dto.AuthCtx) (github.com/stackql/any-sdk/pkg/auth_util.serviceAccount, error)`

### AnySdkClientConfigurator

- receiver: `github.com/stackql/any-sdk/pkg/client.AnySdkClientConfigurator`

- methods:

  - `Auth func(authCtx *github.com/stackql/any-sdk/pkg/dto.AuthCtx, authTypeRequested string, enforceRevokeFirst bool) (github.com/stackql/any-sdk/pkg/client.AnySdkClient, error)`

### AnySdkResponse

- receiver: `github.com/stackql/any-sdk/pkg/client.AnySdkResponse`

- methods:

  - `GetHttpResponse func() (*net/http.Response, error)`

### ControlAttributes

- receiver: `github.com/stackql/any-sdk/pkg/db/sqlcontrol.ControlAttributes`

- methods:

  - `GetControlGCStatusColumnName func() string`
  - `GetControlGenIDColumnName func() string`
  - `GetControlInsIDColumnName func() string`
  - `GetControlInsertEncodedIDColumnName func() string`
  - `GetControlLatestUpdateColumnName func() string`
  - `GetControlMaxTxnColumnName func() string`
  - `GetControlSsnIDColumnName func() string`
  - `GetControlTxnIDColumnName func() string`

### AuthContexts

- receiver: `github.com/stackql/any-sdk/pkg/dto.AuthContexts`

- methods:

  - `Clone func() github.com/stackql/any-sdk/pkg/dto.AuthContexts`

### DataFlowCfg

- receiver: `github.com/stackql/any-sdk/pkg/dto.DataFlowCfg`

- methods:

  - `GetMaxDependencies func() int`

### NamespaceCfg

- receiver: `github.com/stackql/any-sdk/pkg/dto.NamespaceCfg`

- methods:

  - `GetRegex func() (*regexp.Regexp, error)`
  - `GetTemplate func() (*text/template.Template, error)`

### OutputPacket

- receiver: `github.com/stackql/any-sdk/pkg/dto.OutputPacket`

- methods:

  - `GetColumnNames func() []string`
  - `GetColumnOIDs func() []github.com/lib/pq/oid.Oid`
  - `GetRawRows func() map[int]map[int]interface{}`
  - `GetRows func() map[string]map[string]interface{}`

### PgTLSCfg

- receiver: `github.com/stackql/any-sdk/pkg/dto.PgTLSCfg`

- methods:

  - `GetKeyPair func() (crypto/tls.Certificate, error)`

### RuntimeCtx

- receiver: `github.com/stackql/any-sdk/pkg/dto.RuntimeCtx`

- methods:

  - `Copy func() github.com/stackql/any-sdk/pkg/dto.RuntimeCtx`

### SQLBackendCfg

- receiver: `github.com/stackql/any-sdk/pkg/dto.SQLBackendCfg`

- methods:

  - `GetDatabaseName func() (string, error)`
  - `GetIntelViewSchemaName func() string`
  - `GetOpsViewSchemaName func() string`
  - `GetSQLDialect func() string`
  - `GetSchemaType func() string`
  - `GetTableSchemaName func() string`

### SessionContext

- receiver: `github.com/stackql/any-sdk/pkg/dto.SessionContext`

- methods:

  - `GetIsolationLevel func() github.com/stackql/any-sdk/pkg/constants.IsolationLevel`
  - `GetRollbackType func() github.com/stackql/any-sdk/pkg/constants.RollbackType`
  - `UpdateIsolationLevel func(string) error`

### TxnCoordinatorCfg

- receiver: `github.com/stackql/any-sdk/pkg/dto.TxnCoordinatorCfg`

- methods:

  - `GetMaxTxnDepth func() int`

### GQLReader

- receiver: `github.com/stackql/any-sdk/pkg/graphql.GQLReader`

- methods:

  - `Read func() ([]map[string]interface{}, error)`

### ExecPayload

- receiver: `github.com/stackql/any-sdk/pkg/internaldto.ExecPayload`

- methods:

  - `GetPayloadMap func() map[string]interface{}`

### HTTPElement

- receiver: `github.com/stackql/any-sdk/pkg/internaldto.HTTPElement`

- methods:

  - `GetName func() string`
  - `GetType func() github.com/stackql/any-sdk/pkg/internaldto.HTTPElementType`
  - `IsTransformerPresent func() bool`
  - `SetTransformer func(transformer func(interface{}) (interface{}, error))`
  - `Transformer func(interface{}) (interface{}, error)`

### ExecutionResponse

- receiver: `github.com/stackql/any-sdk/pkg/local_template_executor.ExecutionResponse`

- methods:

  - `GetStdErr func() (*bytes.Buffer, bool)`
  - `GetStdOut func() (*bytes.Buffer, bool)`

### Executor

- receiver: `github.com/stackql/any-sdk/pkg/local_template_executor.Executor`

- methods:

  - `Execute func(map[string]any) (github.com/stackql/any-sdk/pkg/local_template_executor.ExecutionResponse, error)`

### NameMangler

- receiver: `github.com/stackql/any-sdk/pkg/name_mangle.NameMangler`

- methods:

  - `MangleName func(string, ...any) string`

### ActionInsertPayload

- receiver: `github.com/stackql/any-sdk/pkg/providerinvoker.ActionInsertPayload`

- methods:

  - `GetItemisationResult func() github.com/stackql/any-sdk/pkg/providerinvoker.ItemisationResult`
  - `GetParamsUsed func() map[string]interface{}`
  - `GetReqEncoding func() string`
  - `GetTableName func() string`
  - `IsHousekeepingDone func() bool`

### ActionInsertResult

- receiver: `github.com/stackql/any-sdk/pkg/providerinvoker.ActionInsertResult`

- methods:

  - `GetError func() (error, bool)`
  - `IsHousekeepingDone func() bool`

### InsertPreparator

- receiver: `github.com/stackql/any-sdk/pkg/providerinvoker.InsertPreparator`

- methods:

  - `ActionInsertPreparation func(payload github.com/stackql/any-sdk/pkg/providerinvoker.ActionInsertPayload) github.com/stackql/any-sdk/pkg/providerinvoker.ActionInsertResult`

### Invoker

- receiver: `github.com/stackql/any-sdk/pkg/providerinvoker.Invoker`

- methods:

  - `Invoke func(ctx context.Context, req github.com/stackql/any-sdk/pkg/providerinvoker.Request) (github.com/stackql/any-sdk/pkg/providerinvoker.Result, error)`

### ItemisationResult

- receiver: `github.com/stackql/any-sdk/pkg/providerinvoker.ItemisationResult`

- methods:

  - `GetItems func() (interface{}, bool)`

### Response

- receiver: `github.com/stackql/any-sdk/pkg/response.Response`

- methods:

  - `Error func() string`
  - `ExtractElement func(e github.com/stackql/any-sdk/pkg/httpelement.HTTPElement) (interface{}, error)`
  - `GetHttpResponse func() *net/http.Response`
  - `GetProcessedBody func() interface{}`
  - `HasError func() bool`

### StreamTransformer

- receiver: `github.com/stackql/any-sdk/pkg/stream_transform.StreamTransformer`

- methods:

  - `GetOutStream func() io.Reader`
  - `Transform func() error`

### StreamTransformerFactory

- receiver: `github.com/stackql/any-sdk/pkg/stream_transform.StreamTransformerFactory`

- methods:

  - `GetTransformer func(input string) (github.com/stackql/any-sdk/pkg/stream_transform.StreamTransformer, error)`
  - `IsTransformable func() bool`

### MapReader

- receiver: `github.com/stackql/any-sdk/pkg/streaming.MapReader`

- methods:

  - `Read func() ([]map[string]interface{}, error)`

### MapStream

- receiver: `github.com/stackql/any-sdk/pkg/streaming.MapStream`

- methods:

  - `Write func([]map[string]interface{}) error`

### MapStreamCollection

- receiver: `github.com/stackql/any-sdk/pkg/streaming.MapStreamCollection`

- methods:

  - `Len func() int`
  - `Push func(github.com/stackql/any-sdk/pkg/streaming.MapStream)`

### AuthDTO

- receiver: `github.com/stackql/any-sdk/pkg/surface.AuthDTO`

- methods:

  - `GetAccountID func() string`
  - `GetAccountIDEnvVar func() string`
  - `GetAuthStyle func() int`
  - `GetClientID func() string`
  - `GetClientIDEnvVar func() string`
  - `GetClientSecret func() string`
  - `GetClientSecretEnvVar func() string`
  - `GetEnvVarAPIKeyStr func() string`
  - `GetEnvVarAPISecretStr func() string`
  - `GetEnvVarPassword func() string`
  - `GetEnvVarUsername func() string`
  - `GetGrantType func() string`
  - `GetInlineBasicCredentials func() string`
  - `GetKeyEnvVar func() string`
  - `GetKeyFilePath func() string`
  - `GetKeyFilePathEnvVar func() string`
  - `GetKeyID func() string`
  - `GetKeyIDEnvVar func() string`
  - `GetLocation func() string`
  - `GetName func() string`
  - `GetScopes func() []string`
  - `GetSubject func() string`
  - `GetSuccessor func() (github.com/stackql/any-sdk/pkg/surface.AuthDTO, bool)`
  - `GetTokenURL func() string`
  - `GetType func() string`
  - `GetValuePrefix func() string`
  - `GetValues func() net/url.Values`

### IDiscoveryAdapter

- receiver: `github.com/stackql/any-sdk/public/discovery.IDiscoveryAdapter`

- methods:

  - `GetProvider func(providerKey string) (github.com/stackql/any-sdk/anysdk.Provider, error)`
  - `GetResourcesMap func(prov github.com/stackql/any-sdk/anysdk.Provider, serviceKey string) (map[string]github.com/stackql/any-sdk/anysdk.Resource, error)`
  - `GetServiceHandlesMap func(prov github.com/stackql/any-sdk/anysdk.Provider) (map[string]github.com/stackql/any-sdk/anysdk.ProviderService, error)`
  - `GetServiceShard func(prov github.com/stackql/any-sdk/anysdk.Provider, serviceKey string, resourceKey string) (github.com/stackql/any-sdk/anysdk.Service, error)`
  - `PersistStaticExternalSQLDataSource func(prov github.com/stackql/any-sdk/anysdk.Provider) error`

### ColumnDescriptor

- receiver: `github.com/stackql/any-sdk/public/formulation.ColumnDescriptor`

- methods:

  - `GetAlias func() string`
  - `GetDecoratedCol func() string`
  - `GetIdentifier func() string`
  - `GetName func() string`
  - `GetNode func() github.com/stackql/stackql-parser/go/vt/sqlparser.SQLNode`
  - `GetQualifier func() string`
  - `GetSchema func() github.com/stackql/any-sdk/anysdk.Schema`
  - `GetVal func() *github.com/stackql/stackql-parser/go/vt/sqlparser.SQLVal`

### AddressSpaceFormulator

- receiver: `github.com/stackql/any-sdk/public/radix_tree_address_space.AddressSpaceFormulator`

- methods:

  - `Formulate func() error`
  - `GetAddressSpace func() github.com/stackql/any-sdk/anysdk.AddressSpace`

### SQLEngine

- receiver: `github.com/stackql/any-sdk/public/sqlengine.SQLEngine`

- methods:

  - `CacheStoreGet func(string) ([]byte, error)`
  - `CacheStorePut func(string, []byte, string, int) error`
  - `Exec func(string, ...interface{}) (database/sql.Result, error)`
  - `ExecInTxn func(queries []string) error`
  - `GetCurrentDiscoveryGenerationID func(discoveryID string) (int, error)`
  - `GetCurrentGenerationID func() (int, error)`
  - `GetDB func() (*database/sql.DB, error)`
  - `GetNextDiscoveryGenerationID func(discoveryID string) (int, error)`
  - `GetNextGenerationID func() (int, error)`
  - `GetNextSessionID func(int) (int, error)`
  - `GetTx func() (*database/sql.Tx, error)`
  - `Query func(string, ...interface{}) (*database/sql.Rows, error)`
  - `QueryRow func(query string, args ...any) *database/sql.Row`

