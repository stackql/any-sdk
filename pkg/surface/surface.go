package surface

import (
	"net/http"
	"net/url"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stackql/any-sdk/pkg/client"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/pkg/internaldto"
	"github.com/stackql/any-sdk/pkg/response"
	"github.com/stackql/stackql-parser/go/sqltypes"
	"github.com/stackql/stackql-parser/go/vt/sqlparser"
)

type Addressable interface {
	ConditionIsValid(lhs string, rhs interface{}) bool
	GetLocation() string
	GetName() string
	GetAlias() string
	GetSchema() (Schema, bool)
	GetType() string
	IsRequired() bool
}

type ColumnDescriptor interface {
	GetAlias() string
	GetDecoratedCol() string
	GetIdentifier() string
	GetName() string
	GetNode() sqlparser.SQLNode
	GetQualifier() string
	GetRepresentativeSchema() Schema
	GetSchema() Schema
	GetVal() *sqlparser.SQLVal
}

type Tabulation interface {
	GetColumns() []ColumnDescriptor
	GetSchema() Schema
	PushBackColumn(col ColumnDescriptor)
	GetName() string
	RenameColumnsToXml() Tabulation
}

type Schema interface {
	SetDefaultColName(string)
	ConditionIsValid(lhs string, rhs interface{}) bool
	DeprecatedProcessHttpResponse(response *http.Response, path string) (map[string]interface{}, error)
	FindByPath(path string, visited map[string]bool) Schema
	GetAdditionalProperties() (Schema, bool)
	GetAllColumns(string) []string
	GetItemProperty(k string) (Schema, bool)
	GetItems() (Schema, error)
	GetItemsSchema() (Schema, error)
	GetName() string
	GetDescription() string
	GetPath() string
	GetProperties() (Schemas, error)
	GetProperty(propertyKey string) (Schema, bool)
	GetSelectionName() string
	GetSelectListItems(key string) (Schema, string)
	GetTitle() string
	GetType() string
	GetPropertySchema(key string) (Schema, error)
	GetRequired() []string
	GetAlias() string
	GetSelectSchema(itemsKey, mediaType string) (Schema, string, error)
	IsArrayRef() bool
	IsBoolean() bool
	IsFloat() bool
	IsIntegral() bool
	IsReadOnly() bool
	IsRequired(key string) bool
	ProcessHttpResponseTesting(r *http.Response, path string, defaultMediaType string, overrideMediaType string) (response.Response, error)
	SetProperties(openapi3.Schemas)
	SetType(string)
	SetKey(string)
	Tabulate(bool, string) Tabulation
	ToDescriptionMap(extended bool) map[string]interface{}
	GetSchemaAtPath(key string, mediaType string) (Schema, error)
}

type Schemas map[string]Schema

type TokenSemanticArgs map[string]interface{}

type TokenTransformer func(interface{}) (interface{}, error)

type TransformerLocator interface {
	GetTransformer(tokenSemantic TokenSemantic) (TokenTransformer, error)
}

type TokenSemantic interface {
	JSONLookup(token string) (interface{}, error)
	GetAlgorithm() string
	GetArgs() TokenSemanticArgs
	GetKey() string
	GetLocation() string
	GetTransformer() (TokenTransformer, error)
	GetProcessedToken(res response.Response) (interface{}, error)
}

type OperationTokens interface {
	JSONLookup(token string) (interface{}, error)
	GetTokenSemantic(key string) (TokenSemantic, bool)
}

type OperationInverse interface {
	JSONLookup(token string) (interface{}, error)
	GetOperationStore() (StandardOperationStore, bool)
	GetTokens() (OperationTokens, bool)
	GetParamMap(response.Response) (map[string]interface{}, error)
}

type AuthDTO interface {
	JSONLookup(token string) (interface{}, error)
	GetInlineBasicCredentials() string
	GetType() string
	GetKeyID() string
	GetKeyIDEnvVar() string
	GetKeyFilePath() string
	GetKeyFilePathEnvVar() string
	GetKeyEnvVar() string
	GetScopes() []string
	GetValuePrefix() string
	GetEnvVarUsername() string
	GetEnvVarPassword() string
	GetEnvVarAPIKeyStr() string
	GetEnvVarAPISecretStr() string
	GetSuccessor() (AuthDTO, bool)
	GetLocation() string
	GetSubject() string
	GetName() string
	GetClientID() string
	GetClientIDEnvVar() string
	GetClientSecret() string
	GetClientSecretEnvVar() string
	GetTokenURL() string
	GetGrantType() string
	GetValues() url.Values
	GetAuthStyle() int
	GetAccountID() string
	GetAccountIDEnvVar() string
}

type Transform interface {
	JSONLookup(token string) (interface{}, error)
	GetAlgorithm() string
	GetType() string
	GetBody() string
}

type View interface {
	GetDDL() string
	GetPredicate() string
	GetNameNaive() string
	GetRequiredParamNames() []string
}

type StackQLConfig interface {
	GetAuth() (AuthDTO, bool)
	GetViewsForSqlDialect(sqlDialect string, viewName string) ([]View, bool)
	GetQueryTranspose() (Transform, bool)
	GetRequestTranslate() (Transform, bool)
	GetRequestBodyTranslate() (Transform, bool)
	GetPagination() (Pagination, bool)
	GetVariations() (Variations, bool)
	GetViews() map[string]View
	GetExternalTables() map[string]SQLExternalTable
	GetQueryParamPushdown() (QueryParamPushdown, bool)
	GetMinStackQLVersion() string
}

type QueryParamPushdown interface {
	JSONLookup(token string) (interface{}, error)
	GetSelect() (SelectPushdown, bool)
	GetFilter() (FilterPushdown, bool)
	GetOrderBy() (OrderByPushdown, bool)
	GetTop() (TopPushdown, bool)
	GetCount() (CountPushdown, bool)
}

type SelectPushdown interface {
	GetDialect() string
	GetParamName() string
	GetDelimiter() string
	GetSupportedColumns() []string
	IsColumnSupported(column string) bool
}

// FilterPushdown represents configuration for WHERE clause filter pushdown
type FilterPushdown interface {
	GetDialect() string
	GetParamName() string
	GetSyntax() string
	GetSupportedOperators() []string
	GetSupportedColumns() []string
	IsOperatorSupported(operator string) bool
	IsColumnSupported(column string) bool
}

// OrderByPushdown represents configuration for ORDER BY clause pushdown
type OrderByPushdown interface {
	GetDialect() string
	GetParamName() string
	GetSyntax() string
	GetSupportedColumns() []string
	IsColumnSupported(column string) bool
}

// TopPushdown represents configuration for LIMIT clause pushdown
type TopPushdown interface {
	GetDialect() string
	GetParamName() string
	GetMaxValue() int
}

// CountPushdown represents configuration for SELECT COUNT(*) pushdown
type CountPushdown interface {
	GetDialect() string
	GetParamName() string
	GetParamValue() string
	GetResponseKey() string
}

type Variations interface {
	JSONLookup(token string) (interface{}, error)
	IsObjectSchemaImplicitlyUnioned() bool
}

type Pagination interface {
	JSONLookup(token string) (interface{}, error)
	GetRequestToken() TokenSemantic
	GetResponseToken() TokenSemantic
}

type GraphQL interface {
	JSONLookup(token string) (interface{}, error)
	GetCursorJSONPath() (string, bool)
	GetResponseJSONPath() (string, bool)
	GetID() string
	GetQuery() string
	GetURL() string
	GetHTTPVerb() string
	GetCursor() GraphQLElement
	GetResponseSelection() GraphQLElement
}

type GraphQLElement map[string]interface{}

type SQLExternalColumn interface {
	GetName() string
	GetType() string
	GetOid() uint32
	GetWidth() int
	GetPrecision() int
}

type SQLExternalTable interface {
	GetCatalogName() string
	GetSchemaName() string
	GetName() string
	GetColumns() []SQLExternalColumn
}

type ExpectedRequest interface {
	GetBodyMediaType() string
	GetSchema() Schema
	GetFinalSchema() Schema
	GetRequired() []string
	GetDefault() string
	GetBase() string
	GetXMLDeclaration() string
	GetXMLTransform() string
}

type ExpectedResponse interface {
	GetBodyMediaType() string
	GetOverrrideBodyMediaType() string
	GetOpenAPIDocKey() string
	GetObjectKey() string
	GetSchema() Schema
	GetProjectionMap() map[string]string
	GetProjection(string) (string, bool)
	GetTransform() (Transform, bool)
}

type OperationStore interface {
	ITable
	GetMethodKey() string
	GetSQLVerb() string
	GetGraphQL() GraphQL
	GetInverse() (OperationInverse, bool)
	GetStackQLConfig() StackQLConfig
	GetQueryParamPushdown() (QueryParamPushdown, bool)
	GetParameters() map[string]Addressable
	GetAPIMethod() string
	GetInline() []string
	GetRequest() (ExpectedRequest, bool)
	GetResponse() (ExpectedResponse, bool)
	GetServers() (openapi3.Servers, bool)
	GetParameterizedPath() string
	GetProviderService() ProviderService
	GetProvider() Provider
	GetService() OpenAPIService
	SetAddressSpace(AddressSpace)
	GetAddressSpace() (AddressSpace, bool)
	GetResource() Resource
	GetProjections() map[string]string
	GetOperationParameter(key string) (Addressable, bool)
	GetSelectSchemaAndObjectPath() (Schema, string, error)
	GetFinalSelectSchemaAndObjectPath() (Schema, string, error)
	ProcessResponse(*http.Response) (ProcessedOperationResponse, error) // to be removed
	GetSelectItemsKey() string
	GetResponseBodySchemaAndMediaType() (Schema, string, error)
	GetFinalResponseBodySchemaAndMediaType() (Schema, string, error)
	GetRequiredParameters() map[string]Addressable
	GetOptionalParameters() map[string]Addressable
	GetParameter(paramKey string) (Addressable, bool)
	GetUnionRequiredParameters() (map[string]Addressable, error)
	GetPaginationResponseTokenSemantic() (TokenSemantic, bool)
	MarshalBody(body interface{}, expectedRequest ExpectedRequest) dto.MarshalledBody
	GetRequestBodySchema() (Schema, error)
	GetNonBodyParameters() map[string]Addressable
	GetRequestBodyAttributesNoRename() (map[string]Addressable, error)
	IsAwaitable() bool
	DeprecatedProcessResponse(response *http.Response) (map[string]interface{}, error)
	GetRequestTranslateAlgorithm() string
	IsRequiredRequestBodyProperty(key string) bool
	GetPaginationRequestTokenSemantic() (TokenSemantic, bool)
	IsNullary() bool
	ToPresentationMap(extended bool) map[string]interface{}
	GetColumnOrder(extended bool) []string
	RenameRequestBodyAttribute(string) (string, error)
	RevertRequestBodyAttributeRename(string) (string, error)
	IsRequestBodyAttributeRenamed(string) bool
	GetRequiredNonBodyParameters() map[string]Addressable
	ShouldBeSelectable() bool
	GetServiceNameForProvider() string
}

type AddressSpaceExpansionConfig interface {
	IsAsync() bool
	IsLegacy() bool
	IsAllowNilResponse() bool
}

type AddressSpace interface {
	GetGlobalSelectSchemas() map[string]Schema
	DereferenceAddress(address string) (any, bool)
	WriteToAddress(address string, val any) error
	ReadFromAddress(address string) (any, bool)
	ResolveSignature(map[string]any) (bool, map[string]any)
	Invoke(...any) error
	ToMap(AddressSpaceExpansionConfig) (map[string]any, error)
	ToRelation(AddressSpaceExpansionConfig) (Relation, error)
}

type HTTPPreparatorConfig interface {
	IsFromAnnotation() bool
}

type HTTPPreparator interface {
	BuildHTTPRequestCtx(HTTPPreparatorConfig) (HTTPArmoury, error)
}

type ParameterBinding interface {
	GetParam() Addressable
	GetVal() interface{}
}

type HttpParameters interface {
	Encode() string
	IngestMap(map[string]interface{}) error
	StoreParameter(Addressable, interface{})
	ToFlatMap() (map[string]interface{}, error)
	GetParameter(paramName, paramIn string) (ParameterBinding, bool)
	GetRemainingQueryParamsFlatMap(keysRemaining map[string]interface{}) (map[string]interface{}, error)
	GetServerParameterFlatMap() (map[string]interface{}, error)
	GetContextParameterFlatMap() (map[string]interface{}, error)
	SetResponseBodyParam(key string, val interface{})
	SetServerParam(key string, svc OpenAPIService, val interface{})
	SetRequestBodyParam(key string, val interface{})
	SetRequestBody(map[string]interface{})
	GetRequestBody() map[string]interface{}
	GetInlineParameterFlatMap() (map[string]interface{}, error)
}

type Service interface {
	IsPreferred() bool
	GetServers() (openapi3.Servers, bool) // Difficult to remove, not impossible.
	GetResources() (map[string]Resource, error)
	GetName() string
	GetResource(resourceName string) (Resource, error)
	GetSchema(key string) (Schema, error)
}

type OperationSelector interface {
	GetSQLVerb() string
	GetParameters() map[string]interface{}
}

type Methods interface {
	FindFromSelector(sel OperationSelector) (StandardOperationStore, error)
	OrderMethods() ([]StandardOperationStore, error)
	FindMethod(key string) (StandardOperationStore, error)
}

type Resource interface {
	ITable
	GetID() string
	GetTitle() string
	GetDescription() string
	GetSelectorAlgorithm() string
	GetMethods() Methods
	GetRequestTranslateAlgorithm() string
	GetPaginationRequestTokenSemantic() (TokenSemantic, bool)
	GetPaginationResponseTokenSemantic() (TokenSemantic, bool)
	GetQueryParamPushdown() (QueryParamPushdown, bool)
	FindMethod(key string) (StandardOperationStore, error)
	GetFirstMethodFromSQLVerb(sqlVerb string) (StandardOperationStore, string, bool)
	GetFirstNamespaceMethodMatchFromSQLVerb(sqlVerb string, parameters map[string]interface{}) (StandardOperationStore, map[string]interface{}, bool)
	GetService() (OpenAPIService, bool)
	GetProvider() (Provider, bool)
	GetViewsForSqlDialect(sqlDialect string) ([]View, bool)
	GetMethodsMatched() Methods
	ToMap(extended bool) map[string]interface{}
}

type ProviderService interface {
	ITable
	GetProvider() (Provider, bool)
	GetProtocolType() (client.ClientProtocolType, error)
	GetService() (Service, error)
	GetRequestTranslateAlgorithm() string
	GetResourcesShallow() (ResourceRegister, error)
	GetPaginationRequestTokenSemantic() (TokenSemantic, bool)
	GetQueryParamPushdown() (QueryParamPushdown, bool)
	ConditionIsValid(lhs string, rhs interface{}) bool
	GetID() string
	GetServiceFragment(resourceKey string) (Service, error)
	GetResourcesRefRef() string
	PeekServiceFragment(resourceKey string) (Service, bool)
	SetServiceRefVal(Service) bool
	IsPreferred() bool
	GetTitle() string
	GetVersion() string
	GetDescription() string
	GetServiceRefRef() string
}

type ResourceRegister interface {
	ObtainServiceDocUrl(resourceKey string) string
	SetProviderService(ps ProviderService)
	SetProvider(p Provider)
	GetResources() map[string]Resource
	GetResource(string) (Resource, bool)
}

type Provider interface {
	GetMinStackQLVersion() string
	GetProtocolType() (client.ClientProtocolType, error)
	GetProtocolTypeString() string
	Debug() string
	GetAuth() (AuthDTO, bool)
	GetDeleteItemsKey() string
	GetName() string
	GetProviderServices() map[string]ProviderService
	GetPaginationRequestTokenSemantic() (TokenSemantic, bool)
	GetPaginationResponseTokenSemantic() (TokenSemantic, bool)
	GetQueryParamPushdown() (QueryParamPushdown, bool)
	GetProviderService(key string) (ProviderService, error)
	GetRequestTranslateAlgorithm() string
	GetResourcesShallow(serviceKey string) (ResourceRegister, error)
	GetStackQLConfig() (StackQLConfig, bool)
	JSONLookup(token string) (interface{}, error)
	MarshalJSON() ([]byte, error)
	UnmarshalJSON(data []byte) error
}

type OpenAPIService interface {
	Service
}

type HTTPArmouryParameters interface {
	Encode() string
	GetBodyBytes() []byte
	GetHeader() http.Header
	GetParameters() HttpParameters
	GetQuery() url.Values
	GetRequest() *http.Request
	GetArgList() client.AnySdkArgList
	SetBodyBytes(b []byte)
	SetHeaderKV(k string, v []string)
	SetNextPage(ops OperationStore, token string, tokenKey internaldto.HTTPElement) (*http.Request, error)
	SetParameters(HttpParameters)
	SetRawQuery(string)
	SetRequest(*http.Request)
	SetRequestBodyMap(BodyMap)
	ToFlatMap() (map[string]interface{}, error)
}

type BodyMap map[string]interface{}

type HTTPArmoury interface {
	AddRequestParams(HTTPArmouryParameters)
	GetRequestParams() []HTTPArmouryParameters
	GetRequestSchema() Schema
	GetResponseSchema() Schema
	SetRequestParams([]HTTPArmouryParameters)
	SetRequestSchema(Schema)
	SetResponseSchema(Schema)
}

type ProcessedOperationResponse interface {
	GetResponse() (response.Response, bool)
	GetReversal() (HTTPPreparator, bool)
}

type StandardOperationStore interface {
	OperationStore
	// Assist analysis
	GetSchemaAtPath(key string) (Schema, error)
	GetSelectItemsKeySimple() string
	LookupSelectItemsKey() string
	//
	GetRequestBodyMediaType() string
	GetRequestBodyMediaTypeNormalised() string
	GetXMLDeclaration() string
	GetXMLRootAnnotation() string
	GetXMLTransform() string
	// getRequestBodyAttributeLineage(string) (string, error)
}

type ITable interface {
	GetName() string
	KeyExists(string) bool
	GetKey(string) (interface{}, error)
	GetKeyAsSqlVal(string) (sqltypes.Value, error)
	GetRequiredParameters() map[string]Addressable
	FilterBy(func(interface{}) (ITable, error)) (ITable, error)
}

type Relation interface {
	GetColumns() []Column
	GetColumnDescriptors() []ColumnDescriptor
}

type Column interface {
	GetName() string
	GetSchema() Schema
	GetWidth() int
}
