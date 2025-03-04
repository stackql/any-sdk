package anysdk

import (
	"fmt"
	"strings"
)

func GetS(in string) string {
	return strings.TrimPrefix(in, "poly")
}

type MethodAnalysisInput interface {
	GetService() OpenAPIService
	GetMethod() OperationStore
	IsNilResponseAllowed() bool
	GetColumns() []ColumnDescriptor
}

type standardMethodAnalysisInput struct {
	method               OperationStore
	service              OpenAPIService
	isNilResponseAllowed bool
	columns              []Column
}

func (mi *standardMethodAnalysisInput) GetMethod() OperationStore {
	return mi.method
}

func (mi *standardMethodAnalysisInput) IsNilResponseAllowed() bool {
	return mi.isNilResponseAllowed
}

type MethodAnalysisOutput interface {
	GetSelectItemsKey() string
	GetInsertTabulation() Tabulation
	GetSelectTabulation() Tabulation
	GetColumns() []ColumnDescriptor
}

type analysisOutput struct {
	selectItemsKey   string
	insertTabulation Tabulation
	selectTabulation Tabulation
	columns          []ColumnDescriptor
}

func (ao *analysisOutput) GetSelectItemsKey() string {
	return ao.selectItemsKey
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
) MethodAnalysisOutput {
	return &analysisOutput{
		selectItemsKey:   selectItemsKey,
		insertTabulation: insertTabulation,
		selectTabulation: selectTabulation,
		columns:          columns,
	}
}

type MethodAnalyzer interface {
	AnalyzeUnaryAction(MethodAnalysisInput) (MethodAnalysisOutput, error)
}

type standardMethodAnalyzer struct{}

func (ma *standardMethodAnalyzer) AnalyzeUnaryAction(
	methodAnalysisInput MethodAnalysisInput,
) (MethodAnalysisOutput, error) {
	method := methodAnalysisInput.GetMethod()
	service := methodAnalysisInput.GetService()
	isNilResponseAllowed := methodAnalysisInput.IsNilResponseAllowed()
	cols := methodAnalysisInput.GetColumns()

	selectItemsKey := method.GetSelectItemsKey()

	schema, mediaType, err := method.GetResponseBodySchemaAndMediaType()
	insertTabulation := newNilTabulation(service, "", "")
	selectTabulation := newNilTabulation(service, "", "")
	if err != nil && !isNilResponseAllowed {
		return nil, err
	}
	if err == nil {
		itemObjS, selectItemsKeyRet, err := schema.GetSelectSchema(method.GetSelectItemsKey(), mediaType)
		if selectItemsKeyRet != "" {
			selectItemsKey = selectItemsKeyRet
		}
		// rscStr, _ := tbl.GetResourceStr()
		unsuitableSchemaMsg := "analyzeUnarySelection(): schema unsuitable for select query"
		if err != nil && !isNilResponseAllowed {
			return nil, err
		}
		// rscStr, _ := tbl.GetResourceStr()
		if itemObjS == nil && !isNilResponseAllowed {
			return nil, fmt.Errorf(unsuitableSchemaMsg)
		}
		if len(cols) == 0 && itemObjS != nil {
			cols = itemObjS.getPropertiesColumns()
		}
		insertTabulation = itemObjS.Tabulate(false, "")

		selectTabulation = itemObjS.Tabulate(true, "")
	}

	return newMethodAnalysisOutput(
		selectItemsKey,
		insertTabulation,
		selectTabulation,
		cols,
	), nil
}
