package anysdk

import (
	"fmt"
	"reflect"

	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stackql/any-sdk/pkg/client"
	"github.com/stackql/stackql-parser/go/sqltypes"
)

var (
	_ ProviderService = &standardProviderService{}
	_ ITable          = &standardProviderService{}
)

type ProviderService interface {
	ITable
	getQueryTransposeAlgorithm() string
	GetProvider() (Provider, bool)
	GetProtocolType() (client.ClientProtocolType, error)
	GetService() (Service, error)
	GetRequestTranslateAlgorithm() string
	GetResourcesShallow() (ResourceRegister, error)
	GetPaginationRequestTokenSemantic() (TokenSemantic, bool)
	getPaginationResponseTokenSemantic() (TokenSemantic, bool)
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
	//
	getResourcesShallowWithRegistry(registry RegistryAPI) (ResourceRegister, error)
	getServiceRefRef() string
	getResourcesRefRef() string
	setService(svc Service) bool
	getServiceWithRegistry(registry RegistryAPI) (Service, error)
	getServiceDocRef(rr ResourceRegister, rsc Resource) ServiceRef
	setProvider(provider Provider)
}

type standardProviderService struct {
	openapi3.ExtensionProps
	ID             string                 `json:"id" yaml:"id"`           // Required
	Name           string                 `json:"name" yaml:"name"`       // Required
	Title          string                 `json:"title" yaml:"title"`     // Required
	Version        string                 `json:"version" yaml:"version"` // Required
	Description    string                 `json:"description" yaml:"description"`
	Preferred      bool                   `json:"preferred" yaml:"preferred"`
	ProtocolType   string                 `json:"protocolType" yaml:"protocolType"`
	ServiceRef     *ServiceRef            `json:"service,omitempty" yaml:"service,omitempty"`     // will be lazy evaluated
	ResourcesRef   *ResourcesRef          `json:"resources,omitempty" yaml:"resources,omitempty"` // will be lazy evaluated
	Provider       Provider               `json:"-" yaml:"-"`                                     // upwards traversal
	StackQLConfig  *standardStackQLConfig `json:"config,omitempty" yaml:"config,omitempty"`
	OpenAPIService OpenAPIService         `json:"-" yaml:"-"`
	GenericService Service                `json:"-" yaml:"-"`
}

func NewEmptyProviderService() ProviderService {
	return &standardProviderService{}
}

func (sv *standardProviderService) GetTitle() string {
	return sv.Title
}

func (sv *standardProviderService) GetProtocolType() (client.ClientProtocolType, error) {
	if sv.ProtocolType != "" {
		return client.ClientProtocolTypeFromString(sv.ProtocolType)
	}
	prov, provOk := sv.GetProvider()
	if provOk {
		return prov.GetProtocolType()
	}
	return client.ClientProtocolTypeFromString(client.ClientProtocolTypeHTTP)
}

func (sv *standardProviderService) GetVersion() string {
	return sv.Version
}

func (sv *standardProviderService) GetDescription() string {
	return sv.Description
}

func (sv *standardProviderService) IsPreferred() bool {
	return sv.Preferred
}

func (sv *standardProviderService) SetServiceRefVal(svc Service) bool {
	switch svc := svc.(type) {
	case *standardService:
		sv.ServiceRef.Value = svc
		return true
	default:
		return false
	}
}

func (sv *standardProviderService) setProvider(provider Provider) {
	sv.Provider = provider
}

func (sv *standardProviderService) GetID() string {
	return sv.ID
}

func (sv *standardProviderService) setService(svc Service) bool {
	openApiSvc, isOpenApiSvc := svc.(OpenAPIService)
	if !isOpenApiSvc {
		return false
	}
	sv.OpenAPIService = openApiSvc
	return true
}

func (sv *standardProviderService) getServiceRefRef() string {
	if sv.ServiceRef == nil {
		return ""
	}
	return sv.ServiceRef.Ref
}

func (sv *standardProviderService) GetResourcesRefRef() string {
	return sv.getResourcesRefRef()
}

func (sv *standardProviderService) getResourcesRefRef() string {
	if sv.ResourcesRef == nil {
		return ""
	}
	return sv.ResourcesRef.Ref
}

func (sv *standardProviderService) GetProvider() (Provider, bool) {
	return sv.Provider, sv.Provider != nil
}

func (sv *standardProviderService) getQueryTransposeAlgorithm() string {
	if sv.StackQLConfig != nil {
		qt, qtExists := sv.StackQLConfig.GetQueryTranspose()
		if qtExists {
			return qt.GetAlgorithm()
		}
	}
	return ""
}

func (sv *standardProviderService) GetRequestTranslateAlgorithm() string {
	if sv.StackQLConfig == nil || sv.StackQLConfig.RequestTranslate == nil {
		return ""
	}
	return sv.StackQLConfig.RequestTranslate.Algorithm
}

func (sv *standardProviderService) GetPaginationRequestTokenSemantic() (TokenSemantic, bool) {
	if sv.StackQLConfig == nil || sv.StackQLConfig.Pagination == nil || sv.StackQLConfig.Pagination.RequestToken == nil {
		return nil, false
	}
	return sv.StackQLConfig.Pagination.RequestToken, true
}

func (sv *standardProviderService) getPaginationResponseTokenSemantic() (TokenSemantic, bool) {
	if sv.StackQLConfig == nil || sv.StackQLConfig.Pagination == nil || sv.StackQLConfig.Pagination.ResponseToken == nil {
		return nil, false
	}
	return sv.StackQLConfig.Pagination.ResponseToken, true
}

func (sv *standardProviderService) ConditionIsValid(lhs string, rhs interface{}) bool {
	elem := sv.ToMap()[lhs]
	return reflect.TypeOf(elem) == reflect.TypeOf(rhs)
}

func extractService(ps ProviderService) (Service, error) {
	b, err := getServiceDocBytes(ps.getServiceRefRef())
	if err != nil {
		return nil, err
	}
	return LoadServiceDocFromBytes(ps, b)
}

func getResourcesShallow(ps ProviderService) (ResourceRegister, error) {
	b, err := getServiceDocBytes(ps.getResourcesRefRef())
	if err != nil {
		return nil, err
	}
	return loadResourcesShallow(ps, b)
}

func (pr *standardProviderService) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(pr)
}

func (pr *standardProviderService) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, pr)
}

func (ps *standardProviderService) FilterBy(predicate func(interface{}) (ITable, error)) (ITable, error) {
	return predicate(ps)
}

func (ps *standardProviderService) ToMap() map[string]interface{} {
	retVal := make(map[string]interface{})
	retVal["id"] = ps.ID
	retVal["name"] = ps.Name
	retVal["title"] = ps.Title
	retVal["description"] = ps.Description
	retVal["version"] = ps.Version
	return retVal
}

func (ps *standardProviderService) GetKeyAsSqlVal(lhs string) (sqltypes.Value, error) {
	val, ok := ps.ToMap()[lhs]
	rv, err := InterfaceToSQLType(val)
	if !ok {
		return rv, fmt.Errorf("key '%s' no preset in providerService", lhs)
	}
	return rv, err
}

func (ps *standardProviderService) GetKey(lhs string) (interface{}, error) {
	val, ok := ps.ToMap()[lhs]
	if !ok {
		return nil, fmt.Errorf("key '%s' no preset in providerService", lhs)
	}
	return val, nil
}

func (ps *standardProviderService) getServiceWithRegistry(registry RegistryAPI) (Service, error) {
	if ps.ServiceRef.Value != nil {
		return ps.ServiceRef.Value, nil
	}
	if registry != nil {
		return registry.GetService(ps)
	}
	svc, err := extractService(ps)
	if err != nil {
		return nil, err
	}
	openapiSvc, isOpenapiSvc := svc.(OpenAPIService)
	if !isOpenapiSvc {
		return nil, fmt.Errorf("disallowed type for openapi service '%T'", svc)
	}
	ps.OpenAPIService = openapiSvc
	return ps.OpenAPIService, nil
}

func (ps *standardProviderService) GetService() (Service, error) {
	if ps.OpenAPIService != nil {
		return ps.OpenAPIService, nil
	}
	if ps.ServiceRef.Value != nil {
		return ps.ServiceRef.Value, nil
	}
	svc, err := extractService(ps)
	if err != nil {
		return nil, err
	}
	openApiSvc, isOpenApiSvc := svc.(OpenAPIService)
	if !isOpenApiSvc {
		return nil, fmt.Errorf("disallowed type for openapi service '%T'", svc)
	}
	ps.OpenAPIService = openApiSvc
	return ps.OpenAPIService, nil
}

func (ps *standardProviderService) extractService() (OpenAPIService, error) {
	if ps.ServiceRef.Value != nil {
		return ps.ServiceRef.Value, nil
	}
	svc, err := extractService(ps)
	if err != nil {
		return nil, err
	}
	openApiSvc, isOpenApiSvc := svc.(OpenAPIService)
	if !isOpenApiSvc {
		return nil, fmt.Errorf("disallowed type for openapi service '%T'", svc)
	}
	ps.OpenAPIService = openApiSvc
	return ps.OpenAPIService, nil
}

func (ps *standardProviderService) getServiceDocRef(rr ResourceRegister, rsc Resource) ServiceRef {
	var rv ServiceRef
	if ps.ServiceRef != nil && ps.ServiceRef.Ref != "" {
		rv = *ps.ServiceRef
	}
	if rr.GetServiceDocPath() != nil && rr.GetServiceDocPath().Ref != "" {
		rv = *(rr.GetServiceDocPath())
	}
	if rsc.GetServiceDocPath() != nil && rsc.GetServiceDocPath().Ref != "" {
		rv = *(rsc.GetServiceDocPath())
	}
	return rv
}

func (ps *standardProviderService) GetServiceFragment(resourceKey string) (Service, error) {

	if ps.ResourcesRef == nil || ps.ResourcesRef.Ref == "" {
		return ps.GetService()
	}
	rr, err := ps.GetResourcesShallow()
	if err != nil {
		return nil, err
	}
	rsc, ok := rr.GetResource(resourceKey)
	if !ok {
		return nil, fmt.Errorf("cannot locate resource for key = '%s'", resourceKey)
	}
	sdRef := ps.getServiceDocRef(rr, rsc)
	if sdRef.Ref == "" {
		return nil, fmt.Errorf("no service doc available for resourceKey = '%s'", resourceKey)
	}
	if sdRef.Value != nil {
		return sdRef.Value, nil
	}
	sb, err := getServiceDocBytes(sdRef.Ref)
	if err != nil {
		return nil, err
	}
	svc, err := LoadServiceSubsetDocFromBytes(rr, resourceKey, sb)
	if err != nil {
		return nil, err
	}
	ps.OpenAPIService = svc
	return ps.OpenAPIService, nil
}

func (ps *standardProviderService) PeekServiceFragment(resourceKey string) (Service, bool) {
	if ps.ServiceRef == nil || ps.ServiceRef.Value == nil || ps.ServiceRef.Value.rsc == nil {
		return nil, false
	}
	_, ok := ps.ServiceRef.Value.rsc[resourceKey]
	if !ok {
		return nil, false
	}
	return ps.ServiceRef.Value, true
}

func (ps *standardProviderService) getResourcesShallowWithRegistry(registry RegistryAPI) (ResourceRegister, error) {
	if ps.ResourcesRef == nil || ps.ResourcesRef.Ref == "" {
		if ps.ServiceRef != nil || ps.ServiceRef.Ref != "" {
			svc, err := ps.getServiceWithRegistry(registry)
			if err != nil {
				return nil, err
			}
			resources, err := svc.GetResources()
			rscCast := make(map[string]*standardResource, len(resources))
			if err != nil {
				return nil, err
			}
			for k, v := range resources {
				rscCast[k] = v.(*standardResource)
			}
			rv := &standardResourceRegister{
				ServiceDocPath: ps.ServiceRef,
				Resources:      rscCast,
			}
			return rv, nil
		}
		return nil, fmt.Errorf("cannot resolve shallow resources")
	}
	if ps.ResourcesRef.Value != nil {
		return ps.ResourcesRef.Value, nil
	}
	if registry != nil {
		return registry.GetResourcesShallowFromURL(ps)
	}
	return getResourcesShallow(ps)
}

func (ps *standardProviderService) GetResourcesShallow() (ResourceRegister, error) {
	if ps.ResourcesRef == nil || ps.ResourcesRef.Ref == "" {
		if ps.ServiceRef != nil || ps.ServiceRef.Ref != "" {
			svc, err := ps.GetService()
			if err != nil {
				return nil, err
			}
			resources, err := svc.GetResources()
			if err != nil {
				return nil, err
			}
			rscCast := make(map[string]*standardResource, len(resources))
			for k, v := range resources {
				rscCast[k] = v.(*standardResource)
			}
			rv := &standardResourceRegister{
				ServiceDocPath: ps.ServiceRef,
				Resources:      rscCast,
			}
			return rv, nil
		}
		return nil, fmt.Errorf("cannot resolve shallow resources")
	}
	if ps.ResourcesRef.Value != nil {
		return ps.ResourcesRef.Value, nil
	}
	return getResourcesShallow(ps)
}

func (ps *standardProviderService) GetName() string {
	return ps.Name
}

func (ps *standardProviderService) GetRequiredParameters() map[string]Addressable {
	return nil
}

func (ps *standardProviderService) KeyExists(lhs string) bool {
	_, ok := ps.ToMap()[lhs]
	return ok
}
