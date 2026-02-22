package formulation

import (
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/stackql/any-sdk/anysdk"
	"github.com/stackql/any-sdk/pkg/client"
	"github.com/stackql/any-sdk/pkg/dto"
	"github.com/stackql/any-sdk/pkg/internaldto"
	"github.com/stackql/any-sdk/pkg/providerinvoker"
	"github.com/stackql/any-sdk/pkg/streaming"
	"github.com/stackql/any-sdk/public/discovery"
	"github.com/stackql/any-sdk/public/persistence"
	"github.com/stackql/any-sdk/public/providerinvokers/anysdkhttp"
	"github.com/stackql/any-sdk/public/radix_tree_address_space"
	"github.com/stackql/any-sdk/public/sqlengine"
	"github.com/stackql/stackql-parser/go/vt/sqlparser"
)

func NewSQLPersistenceSystem(systemType string, sqlEngine sqlengine.SQLEngine) (PersistenceSystem, error) {
	anySdkPersistenceSystem, err := persistence.NewSQLPersistenceSystem(systemType, sqlEngine)
	if err != nil {
		return nil, err
	}
	return &wrappedPersistenceSystem{inner: anySdkPersistenceSystem}, nil
}

// type Addressable interface {
// 	ConditionIsValid(lhs string, rhs interface{}) bool
// 	GetLocation() string
// 	GetName() string
// 	GetAlias() string
// 	GetSchema() (anysdk.Schema, bool)
// 	GetType() string
// 	IsRequired() bool
// }

// type ITable interface {
// 	GetName() string
// 	KeyExists(string) bool
// 	GetKey(string) (interface{}, error)
// 	GetKeyAsSqlVal(string) (sqltypes.Value, error)
// 	GetRequiredParameters() map[string]Addressable
// 	FilterBy(func(interface{}) (ITable, error)) (ITable, error)
// }

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
	unwrap() anysdk.ColumnDescriptor
}

type PersistenceSystem interface {
	GetSystemName() string
	HandleExternalTables(providerName string, externalTables map[string]SQLExternalTable) error
	HandleViewCollection([]View) error
	CacheStoreGet(key string) ([]byte, error)
	CacheStorePut(key string, value []byte, expiration string, ttl int) error
	// unwrap() persistence.PersistenceSystem
}

func NewColumnDescriptor(alias string, name string, qualifier string, decoratedCol string, node sqlparser.SQLNode, schema Schema, val *sqlparser.SQLVal) ColumnDescriptor {
	rv := anysdk.NewColumnDescriptor(alias, name, qualifier, decoratedCol, node, schema.unwrap(), val)
	return newColDescriptorFromAnySdkColumnDescriptor(rv)
}

func NewMethodAnalysisInput(
	method OperationStore,
	service Service,
	isNilResponseAllowed bool,
	columns []ColumnDescriptor,
	isAwait bool,
) anysdk.MethodAnalysisInput {
	cols := make([]anysdk.ColumnDescriptor, len(columns))
	for i, c := range columns {
		cols[i] = c.unwrap()
	}
	return anysdk.NewMethodAnalysisInput(
		method.unwrap(),
		service.unwrap(),
		isNilResponseAllowed,
		cols,
		isAwait,
	)
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
	return newHTTPPreparatorFromAnySdkHTTPPreparator(
		anysdk.NewHTTPPreparator(
			prov.unwrap(),
			svc.unwrap(),
			m.unwrap(),
			paramMap,
			parameters,
			execContext.unwrap(),
			logger,
		),
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
	return anysdk.CallFromSignature(
		cc,
		runtimeCtx,
		authCtx,
		authTypeRequested,
		enforceRevokeFirst,
		outErrFile,
		prov.unwrap(),
		designation,
		argList,
	)
}

func NewAnySdkOpStoreDesignation(method OperationStore) client.AnySdkDesignation {
	return anysdk.NewAnySdkOpStoreDesignation(method.unwrap())
}

func NewRegistry(registryCfg RegistryConfig, transport http.RoundTripper) (RegistryAPI, error) {
	rv, err := anysdk.NewRegistry(registryCfg.toAnySdkRegistryConfig(), transport)
	if err != nil {
		return nil, err
	}
	return &wrappedRegistryAPI{inner: rv}, nil
}

var DefaultLinkHeaderTransformer = anysdk.DefaultLinkHeaderTransformer

var NewAnySdkClientConfigurator = anysdk.NewAnySdkClientConfigurator

func NewStringSchema(svc OpenAPIService, key string, path string) Schema {
	raw := anysdk.NewStringSchema(svc.unwrapOpenapi3Service(), key, path)
	return newWrappedSchemaFromAnySdkSchema(raw)
}

func NewStandardAddressSpaceExpansionConfig(
	isAsync bool,
	isLegacy bool,
	isAllowNilResponse bool,
) AddressSpaceExpansionConfig {
	return &wrappedAddressSpaceExpansionConfig{
		inner: radix_tree_address_space.NewStandardAddressSpaceExpansionConfig(isAsync, isLegacy, isAllowNilResponse),
	}
}

func LoadProviderAndServiceFromPaths(
	provFilePath string,
	svcFilePath string,
) (Service, error) {
	svc, err := anysdk.LoadProviderAndServiceFromPaths(provFilePath, svcFilePath)
	if err != nil {
		return nil, err
	}
	return &wrappedService{inner: svc}, nil
}

func NewAddressSpaceFormulator(
	grammar AddressSpaceGrammar,
	provider Provider,
	service Service,
	resource Resource,
	method StandardOperationStore,
	aliasedUnionSelectKeys map[string]string,
	isAwait bool,
) AddressSpaceFormulator {
	return &wrappedAddressSpaceFormulator{
		inner: radix_tree_address_space.NewAddressSpaceFormulator(
			grammar,
			provider.unwrap(),
			service.unwrap(),
			resource.unwrap(),
			method.unwrapStandardOperationStore(),
			aliasedUnionSelectKeys,
			isAwait,
		),
	}
}

func NewBasicDiscoveryAdapter(
	alias string,
	apiDiscoveryDocURL string,
	discoveryStore IDiscoveryStore,
	runtimeCtx *dto.RuntimeCtx,
	registry RegistryAPI,
	persistenceSystem PersistenceSystem,
) IDiscoveryAdapter {
	reverseWrappedSystem := &reverseWrappedPersistenceSystem{inner: persistenceSystem}
	return &wrappedDiscoveryAdapter{
		inner: discovery.NewBasicDiscoveryAdapter(
			alias,
			apiDiscoveryDocURL,
			discoveryStore.unwrap(),
			runtimeCtx,
			registry.unwrap(),
			reverseWrappedSystem,
		),
	}
}

type AuthMetadata struct {
	Principal string `json:"principal"`
	Type      string `json:"type"`
	Source    string `json:"source"`
}

func (am *AuthMetadata) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"principal": am.Principal,
		"type":      am.Type,
		"source":    am.Source,
	}
}

func (am *AuthMetadata) GetHeaders() []string {
	return []string{
		"principal",
		"type",
		"source",
	}
}

func NewMethodAnalyzer() MethodAnalyzer {
	return &wrappedMethodAnalyzer{}
}

func NewHTTPPreparatorConfig(isFromAnnotation bool) HTTPPreparatorConfig {
	return &wrappedHTTPPreparatorConfig{
		inner: anysdk.NewHTTPPreparatorConfig(isFromAnnotation),
	}
}

func EmptyMethods() Methods {
	return &wrappedMethods{inner: anysdk.Methods{}}
}

func GetServicesHeader(extended bool) []string {
	var retVal []string
	if extended {
		retVal = []string{
			"id",
			"name",
			"title",
			"description",
			"version",
			"preferred",
		}
	} else {
		retVal = []string{
			"id",
			"name",
			"title",
		}
	}
	return retVal
}

func GetDescribeHeader(extended bool) []string {
	var retVal []string
	if extended {
		retVal = []string{
			"name",
			"type",
			"description",
		}
	} else {
		retVal = []string{
			"name",
			"type",
		}
	}
	return retVal
}

func GetResourcesHeader(extended bool) []string {
	var retVal []string
	if extended {
		retVal = []string{
			"name",
			"id",
			"description",
		}
	} else {
		retVal = []string{
			"name",
			"id",
		}
	}
	return retVal
}

func NewTTLDiscoveryStore(
	persistenceSystem PersistenceSystem,
	registry RegistryAPI,
	runtimeCtx dto.RuntimeCtx,
) IDiscoveryStore {
	reverseWrappedSystem := &reverseWrappedPersistenceSystem{inner: persistenceSystem}
	return &wrappedTTLDiscoveryStore{
		inner: discovery.NewTTLDiscoveryStore(
			reverseWrappedSystem,
			registry.unwrap(),
			runtimeCtx,
		),
	}
}

func NewAddressSpaceGrammar() AddressSpaceGrammar {
	return radix_tree_address_space.NewAddressSpaceGrammar()
}

func ResourceKeyExists(key string) bool {
	return anysdk.ResourceKeyExists(key)
}

func NewEmptyResource() Resource {
	return &wrappedResource{inner: anysdk.NewEmptyResource()}
}

func NewEmptyOperationStore() StandardOperationStore {
	return &wrappedStandardOperationStore{inner: anysdk.NewEmptyOperationStore()}
}

func NewExecContext(payload internaldto.ExecPayload, rsc Resource) ExecContext {
	return &wrappedExecContext{
		inner: anysdk.NewExecContext(payload, rsc.unwrap()),
	}
}

func NewEmptyProviderService() ProviderService {
	return &wrappedProviderService{inner: anysdk.NewEmptyProviderService()}
}

func ServiceKeyExists(key string) bool {
	return anysdk.ServiceKeyExists(key)
}

func NewwHTTPAnySdkArgList(req *http.Request) client.AnySdkArgList {
	return anysdk.NewwHTTPAnySdkArgList(req)
}

func NewHttpPreparatorStream() HttpPreparatorStream {
	return &wrappedHttpPreparatorStream{
		inner: anysdk.NewHttpPreparatorStream(),
	}
}

func GetMonitorRequest(urlStr string) (client.AnySdkArgList, error) {
	return anysdk.GetMonitorRequest(urlStr)
}

type methodElider interface {
	IsElide(string, ...any) bool
}

type PolyHandler interface {
	LogHTTPResponseMap(target interface{})
	MessageHandler([]string)
	GetMessages() []string
}

type BaseArmouryGenerator interface {
	GetHTTPArmoury() (HTTPArmoury, error)
}

type ArmouryGenerator interface {
	BaseArmouryGenerator
	unwrap() anysdkhttp.ArmouryGenerator
}

type wrappedArmouryGenerator struct {
	inner anysdkhttp.ArmouryGenerator
}

func (wag *wrappedArmouryGenerator) GetHTTPArmoury() (HTTPArmoury, error) {
	inner, err := wag.inner.GetHTTPArmoury()
	if err != nil {
		return nil, err
	}
	return &wrappedHTTPArmoury{inner: inner}, nil
}

func (wag *wrappedArmouryGenerator) unwrap() anysdkhttp.ArmouryGenerator {
	return wag.inner
}

// anysdkhttp.ArmouryGenerator

func NewPayload(
	armouryGenerator BaseArmouryGenerator,
	provider Provider,
	method OperationStore,
	tableName string,
	authCtx *dto.AuthCtx,
	runtimeCtx dto.RuntimeCtx,
	outErrFile io.Writer,
	maxResultsElement internaldto.HTTPElement,
	elider methodElider,
	nilOK bool,
	polyHandler PolyHandler,
	selectItemsKey string,
	insertPreparator BaseInsertPreparator,
	skipResponse bool,
	isMutation bool,
	isAwait bool,
	defaultHTTPClient *http.Client,
	messageHandler providerinvoker.MessageHandler,
) any {
	return anysdkhttp.NewPayload(
		nil, // placeholder for armoury generator, as the public formulation layer should not be aware of anysdkhttp
		provider.unwrap(),
		method.unwrap(),
		tableName,
		authCtx,
		runtimeCtx,
		outErrFile,
		maxResultsElement,
		elider,
		nilOK,
		polyHandler,
		selectItemsKey,
		&reverseWrappedInsertPreparator{inner: insertPreparator},
		skipResponse,
		isMutation,
		isAwait,
		defaultHTTPClient,
		messageHandler,
	)
}

func NewActionInsertPayload(
	itemisationResult ItemisationResult,
	housekeepingDone bool,
	tableName string,
	paramsUsed map[string]interface{},
	reqEncoding string,
) ActionInsertPayload {
	return &wrappedActionInsertPayload{
		inner: &httpActionInsertPayload{
			itemisationResult: itemisationResult,
			housekeepingDone:  housekeepingDone,
			tableName:         tableName,
			paramsUsed:        paramsUsed,
			reqEncoding:       reqEncoding,
		},
	}
}

func LoadProviderDocFromBytes(bytes []byte) (Provider, error) {
	prov, err := anysdk.LoadProviderDocFromBytes(bytes)
	if err != nil {
		return nil, err
	}
	return &wrappedProvider{inner: prov}, nil
}

func ServiceConditionIsValid(lhs string, rhs interface{}) bool {
	return anysdk.ServiceConditionIsValid(lhs, rhs)
}

func ResourceConditionIsValid(lhs string, rhs interface{}) bool {
	return anysdk.ResourceConditionIsValid(lhs, rhs)
}
