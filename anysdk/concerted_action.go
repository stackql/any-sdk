package anysdk

import (
	"fmt"
	"sort"
)

type MethodAnalysisInput interface {
	GetService() Service
	GetMethod() OperationStore
	IsAwait() bool
	IsNilResponseAllowed() bool
	GetColumns() []ColumnDescriptor
}

type standardMethodAnalysisInput struct {
	method               OperationStore
	service              Service
	isNilResponseAllowed bool
	columns              []ColumnDescriptor
	isAwait              bool
}

func NewMethodAnalysisInput(
	method OperationStore,
	service Service,
	isNilResponseAllowed bool,
	columns []ColumnDescriptor,
	isAwait bool,
) MethodAnalysisInput {
	return &standardMethodAnalysisInput{
		method:               method,
		service:              service,
		isNilResponseAllowed: isNilResponseAllowed,
		columns:              columns,
		isAwait:              isAwait,
	}
}

func (mi *standardMethodAnalysisInput) IsAwait() bool {
	return mi.isAwait
}

func (mi *standardMethodAnalysisInput) GetMethod() OperationStore {
	return mi.method
}

func (mi *standardMethodAnalysisInput) GetService() Service {
	return mi.service
}

func (mi *standardMethodAnalysisInput) GetColumns() []ColumnDescriptor {
	return mi.columns
}

func (mi *standardMethodAnalysisInput) IsNilResponseAllowed() bool {
	return mi.isNilResponseAllowed
}

type MethodAnalysisOutput interface {
	GetMethod() OperationStore
	GetSelectItemsKey() string
	GetInsertTabulation() Tabulation
	GetSelectTabulation() Tabulation
	GetColumns() []ColumnDescriptor
	GetStarColumns() (Schemas, error)
	GetOrderedStarColumnsNames() ([]string, error)
	GetItemSchema() (Schema, bool)
	GetResponseSchema() (Schema, bool)
	IsNilResponseAllowed() bool
}

type analysisOutput struct {
	method               OperationStore
	selectItemsKey       string
	insertTabulation     Tabulation
	selectTabulation     Tabulation
	columns              []ColumnDescriptor
	responseSchema       Schema
	itemSchema           Schema
	isNilResponseAllowed bool
}

func (ao *analysisOutput) GetMethod() OperationStore {
	return ao.method
}

func (ao *analysisOutput) IsNilResponseAllowed() bool {
	return ao.isNilResponseAllowed
}

func (ao *analysisOutput) GetSelectItemsKey() string {
	return ao.selectItemsKey
}

func (ao *analysisOutput) GetItemSchema() (Schema, bool) {
	return ao.itemSchema, ao.itemSchema != nil
}

func (ao *analysisOutput) GetResponseSchema() (Schema, bool) {
	return ao.responseSchema, ao.responseSchema != nil
}

func (ao *analysisOutput) GetInsertTabulation() Tabulation {
	return ao.insertTabulation
}

func (ao *analysisOutput) GetSelectTabulation() Tabulation {
	return ao.selectTabulation
}

func (ao *analysisOutput) GetColumns() []ColumnDescriptor {
	return ao.columns
}

func (ao *analysisOutput) GetOrderedStarColumnsNames() ([]string, error) {
	colSchemas, err := ao.GetStarColumns()
	if err != nil {
		return nil, fmt.Errorf("GetOrderedStarColumnsNames(): %w", err)
	}
	var cols []string
	for colName, _ := range colSchemas {
		if err != nil {
			return nil, fmt.Errorf("GetOrderedStarColumnsNames(): %w", err)
		}
		cols = append(cols, colName)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("GetOrderedStarColumnsNames(): no columns found")
	}
	// Sort columns lexicographically
	sort.Strings(cols)
	return cols, nil
}

func (ao *analysisOutput) GetStarColumns() (Schemas, error) {
	if ao.itemSchema == nil {
		return nil, fmt.Errorf("GetStarColumns(): itemSchema is nil")
	}
	return ao.itemSchema.GetProperties()
}

func newMethodAnalysisOutput(
	method OperationStore,
	selectItemsKey string,
	insertTabulation Tabulation,
	selectTabulation Tabulation,
	columns []ColumnDescriptor,
	responseSchema Schema,
	itemSchema Schema,
	isNilResponseAllowed bool,
) MethodAnalysisOutput {
	return &analysisOutput{
		method:               method,
		selectItemsKey:       selectItemsKey,
		insertTabulation:     insertTabulation,
		selectTabulation:     selectTabulation,
		columns:              columns,
		responseSchema:       responseSchema,
		itemSchema:           itemSchema,
		isNilResponseAllowed: isNilResponseAllowed,
	}
}

func NewMethodAnalyzer() MethodAnalyzer {
	return &standardMethodAnalyzer{}
}

type MethodAnalyzer interface {
	AnalyzeUnaryAction(MethodAnalysisInput) (MethodAnalysisOutput, error)
}

type standardMethodAnalyzer struct{}

func (ma *standardMethodAnalyzer) AnalyzeUnaryAction(
	methodAnalysisInput MethodAnalysisInput,
) (MethodAnalysisOutput, error) {
	method := methodAnalysisInput.GetMethod()
	svc := methodAnalysisInput.GetService()
	service, serviceOk := svc.(OpenAPIService)
	if !serviceOk {
		return nil, fmt.Errorf("AnalyzeUnaryAction(): service is not an OpenAPIService")
	}
	isNilResponseAllowed := methodAnalysisInput.IsNilResponseAllowed()
	cols := methodAnalysisInput.GetColumns()

	selectItemsKey := method.GetSelectItemsKey()

	isAwait := methodAnalysisInput.IsAwait()

	var schema Schema
	var mediaType string
	var err error

	if isAwait {
		schema, mediaType, err = method.GetFinalResponseBodySchemaAndMediaType()
	} else {
		schema, mediaType, err = method.GetResponseBodySchemaAndMediaType()
	}
	insertTabulation := newNilTabulation(service, "", "")
	selectTabulation := newNilTabulation(service, "", "")
	if err != nil && !isNilResponseAllowed {
		return nil, err
	}
	if err != nil {
		schema = newExmptyObjectStandardSchema(service, "", "")
	}
	itemSchema := schema
	if err == nil {
		itemObjS, selectItemsKeyRet, err := schema.GetSelectSchema(method.GetSelectItemsKey(), mediaType)
		if selectItemsKeyRet != "" {
			selectItemsKey = selectItemsKeyRet
		}
		itemSchema = itemObjS
		// rscStr, _ := tbl.GetResourceStr()
		unsuitableSchemaMsg := "analyzeUnarySelection(): schema unsuitable for select query"
		if err != nil && !isNilResponseAllowed {
			return nil, err
		}
		// rscStr, _ := tbl.GetResourceStr()
		if itemObjS == nil && !isNilResponseAllowed {
			return nil, fmt.Errorf("%s", unsuitableSchemaMsg)
		}
		if len(cols) == 0 && itemObjS != nil {
			cols = itemObjS.getPropertiesColumns()
			// TODO: order
		}
		insertTabulation = itemObjS.Tabulate(false, "")

		selectTabulation = itemObjS.Tabulate(true, "")
	}

	return newMethodAnalysisOutput(
		method,
		selectItemsKey,
		insertTabulation,
		selectTabulation,
		cols,
		schema,
		itemSchema,
		isNilResponseAllowed,
	), nil
}
