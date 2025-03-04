package anysdk

import (
	"fmt"
)

type MethodAnalysisInput interface {
	GetService() Service
	GetMethod() OperationStore
	IsNilResponseAllowed() bool
	GetColumns() []ColumnDescriptor
}

type standardMethodAnalysisInput struct {
	method               OperationStore
	service              Service
	isNilResponseAllowed bool
	columns              []ColumnDescriptor
}

func NewMethodAnalysisInput(
	method OperationStore,
	service Service,
	isNilResponseAllowed bool,
	columns []ColumnDescriptor,
) MethodAnalysisInput {
	return &standardMethodAnalysisInput{
		method:               method,
		service:              service,
		isNilResponseAllowed: isNilResponseAllowed,
		columns:              columns,
	}
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
	GetSelectItemsKey() string
	GetInsertTabulation() Tabulation
	GetSelectTabulation() Tabulation
	GetColumns() []ColumnDescriptor
	GetItemSchema() (Schema, bool)
	GetResponseSchema() (Schema, bool)
}

type analysisOutput struct {
	selectItemsKey   string
	insertTabulation Tabulation
	selectTabulation Tabulation
	columns          []ColumnDescriptor
	responseSchema   Schema
	itemSchema       Schema
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

func newMethodAnalysisOutput(
	selectItemsKey string,
	insertTabulation Tabulation,
	selectTabulation Tabulation,
	columns []ColumnDescriptor,
	responseSchema Schema,
	itemSchema Schema,
) MethodAnalysisOutput {
	return &analysisOutput{
		selectItemsKey:   selectItemsKey,
		insertTabulation: insertTabulation,
		selectTabulation: selectTabulation,
		columns:          columns,
		responseSchema:   responseSchema,
		itemSchema:       itemSchema,
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

	schema, mediaType, err := method.GetResponseBodySchemaAndMediaType()
	insertTabulation := newNilTabulation(service, "", "")
	selectTabulation := newNilTabulation(service, "", "")
	if err != nil && !isNilResponseAllowed {
		return nil, err
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
		selectItemsKey,
		insertTabulation,
		selectTabulation,
		cols,
		schema,
		itemSchema,
	), nil
}
