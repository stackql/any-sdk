package anysdk

import (
	"fmt"

	"github.com/go-openapi/jsonpointer"
)

// OData default values
const (
	ODataDialect             = "odata"
	CustomDialect            = "custom"
	DefaultSelectParamName   = "$select"
	DefaultSelectDelimiter   = ","
	DefaultFilterParamName   = "$filter"
	DefaultFilterSyntax      = "odata"
	DefaultOrderByParamName  = "$orderby"
	DefaultOrderBySyntax     = "odata"
	DefaultTopParamName      = "$top"
	DefaultCountParamName    = "$count"
	DefaultCountParamValue   = "true"
	DefaultCountResponseKey  = "@odata.count"
)

var (
	_ QueryParamPushdown        = &standardQueryParamPushdown{}
	_ jsonpointer.JSONPointable = standardQueryParamPushdown{}
	_ SelectPushdown            = &standardSelectPushdown{}
	_ FilterPushdown            = &standardFilterPushdown{}
	_ OrderByPushdown           = &standardOrderByPushdown{}
	_ TopPushdown               = &standardTopPushdown{}
	_ CountPushdown             = &standardCountPushdown{}
)

// QueryParamPushdown represents the top-level configuration for query parameter pushdown
type QueryParamPushdown interface {
	JSONLookup(token string) (interface{}, error)
	GetSelect() (SelectPushdown, bool)
	GetFilter() (FilterPushdown, bool)
	GetOrderBy() (OrderByPushdown, bool)
	GetTop() (TopPushdown, bool)
	GetCount() (CountPushdown, bool)
}

// SelectPushdown represents configuration for SELECT clause column projection pushdown
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

// standardQueryParamPushdown is the concrete implementation
type standardQueryParamPushdown struct {
	Select  *standardSelectPushdown  `json:"select,omitempty" yaml:"select,omitempty"`
	Filter  *standardFilterPushdown  `json:"filter,omitempty" yaml:"filter,omitempty"`
	OrderBy *standardOrderByPushdown `json:"orderBy,omitempty" yaml:"orderBy,omitempty"`
	Top     *standardTopPushdown     `json:"top,omitempty" yaml:"top,omitempty"`
	Count   *standardCountPushdown   `json:"count,omitempty" yaml:"count,omitempty"`
}

func (qpp standardQueryParamPushdown) JSONLookup(token string) (interface{}, error) {
	switch token {
	case "select":
		return qpp.Select, nil
	case "filter":
		return qpp.Filter, nil
	case "orderBy":
		return qpp.OrderBy, nil
	case "top":
		return qpp.Top, nil
	case "count":
		return qpp.Count, nil
	default:
		return nil, fmt.Errorf("could not resolve token '%s' from QueryParamPushdown doc object", token)
	}
}

func (qpp *standardQueryParamPushdown) GetSelect() (SelectPushdown, bool) {
	if qpp.Select == nil {
		return nil, false
	}
	return qpp.Select, true
}

func (qpp *standardQueryParamPushdown) GetFilter() (FilterPushdown, bool) {
	if qpp.Filter == nil {
		return nil, false
	}
	return qpp.Filter, true
}

func (qpp *standardQueryParamPushdown) GetOrderBy() (OrderByPushdown, bool) {
	if qpp.OrderBy == nil {
		return nil, false
	}
	return qpp.OrderBy, true
}

func (qpp *standardQueryParamPushdown) GetTop() (TopPushdown, bool) {
	if qpp.Top == nil {
		return nil, false
	}
	return qpp.Top, true
}

func (qpp *standardQueryParamPushdown) GetCount() (CountPushdown, bool) {
	if qpp.Count == nil {
		return nil, false
	}
	return qpp.Count, true
}

// standardSelectPushdown implements SelectPushdown
type standardSelectPushdown struct {
	Dialect          string   `json:"dialect,omitempty" yaml:"dialect,omitempty"`
	ParamName        string   `json:"paramName,omitempty" yaml:"paramName,omitempty"`
	Delimiter        string   `json:"delimiter,omitempty" yaml:"delimiter,omitempty"`
	SupportedColumns []string `json:"supportedColumns,omitempty" yaml:"supportedColumns,omitempty"`
}

func (sp *standardSelectPushdown) GetDialect() string {
	if sp.Dialect == "" {
		return CustomDialect
	}
	return sp.Dialect
}

func (sp *standardSelectPushdown) GetParamName() string {
	if sp.ParamName == "" && sp.GetDialect() == ODataDialect {
		return DefaultSelectParamName
	}
	return sp.ParamName
}

func (sp *standardSelectPushdown) GetDelimiter() string {
	if sp.Delimiter == "" {
		if sp.GetDialect() == ODataDialect {
			return DefaultSelectDelimiter
		}
		return ","
	}
	return sp.Delimiter
}

func (sp *standardSelectPushdown) GetSupportedColumns() []string {
	return sp.SupportedColumns
}

func (sp *standardSelectPushdown) IsColumnSupported(column string) bool {
	return isItemSupported(column, sp.SupportedColumns)
}

// standardFilterPushdown implements FilterPushdown
type standardFilterPushdown struct {
	Dialect            string   `json:"dialect,omitempty" yaml:"dialect,omitempty"`
	ParamName          string   `json:"paramName,omitempty" yaml:"paramName,omitempty"`
	Syntax             string   `json:"syntax,omitempty" yaml:"syntax,omitempty"`
	SupportedOperators []string `json:"supportedOperators,omitempty" yaml:"supportedOperators,omitempty"`
	SupportedColumns   []string `json:"supportedColumns,omitempty" yaml:"supportedColumns,omitempty"`
}

func (fp *standardFilterPushdown) GetDialect() string {
	if fp.Dialect == "" {
		return CustomDialect
	}
	return fp.Dialect
}

func (fp *standardFilterPushdown) GetParamName() string {
	if fp.ParamName == "" && fp.GetDialect() == ODataDialect {
		return DefaultFilterParamName
	}
	return fp.ParamName
}

func (fp *standardFilterPushdown) GetSyntax() string {
	if fp.Syntax == "" && fp.GetDialect() == ODataDialect {
		return DefaultFilterSyntax
	}
	return fp.Syntax
}

func (fp *standardFilterPushdown) GetSupportedOperators() []string {
	return fp.SupportedOperators
}

func (fp *standardFilterPushdown) GetSupportedColumns() []string {
	return fp.SupportedColumns
}

func (fp *standardFilterPushdown) IsOperatorSupported(operator string) bool {
	return isItemSupported(operator, fp.SupportedOperators)
}

func (fp *standardFilterPushdown) IsColumnSupported(column string) bool {
	return isItemSupported(column, fp.SupportedColumns)
}

// standardOrderByPushdown implements OrderByPushdown
type standardOrderByPushdown struct {
	Dialect          string   `json:"dialect,omitempty" yaml:"dialect,omitempty"`
	ParamName        string   `json:"paramName,omitempty" yaml:"paramName,omitempty"`
	Syntax           string   `json:"syntax,omitempty" yaml:"syntax,omitempty"`
	SupportedColumns []string `json:"supportedColumns,omitempty" yaml:"supportedColumns,omitempty"`
}

func (op *standardOrderByPushdown) GetDialect() string {
	if op.Dialect == "" {
		return CustomDialect
	}
	return op.Dialect
}

func (op *standardOrderByPushdown) GetParamName() string {
	if op.ParamName == "" && op.GetDialect() == ODataDialect {
		return DefaultOrderByParamName
	}
	return op.ParamName
}

func (op *standardOrderByPushdown) GetSyntax() string {
	if op.Syntax == "" && op.GetDialect() == ODataDialect {
		return DefaultOrderBySyntax
	}
	return op.Syntax
}

func (op *standardOrderByPushdown) GetSupportedColumns() []string {
	return op.SupportedColumns
}

func (op *standardOrderByPushdown) IsColumnSupported(column string) bool {
	return isItemSupported(column, op.SupportedColumns)
}

// standardTopPushdown implements TopPushdown
type standardTopPushdown struct {
	Dialect   string `json:"dialect,omitempty" yaml:"dialect,omitempty"`
	ParamName string `json:"paramName,omitempty" yaml:"paramName,omitempty"`
	MaxValue  int    `json:"maxValue,omitempty" yaml:"maxValue,omitempty"`
}

func (tp *standardTopPushdown) GetDialect() string {
	if tp.Dialect == "" {
		return CustomDialect
	}
	return tp.Dialect
}

func (tp *standardTopPushdown) GetParamName() string {
	if tp.ParamName == "" && tp.GetDialect() == ODataDialect {
		return DefaultTopParamName
	}
	return tp.ParamName
}

func (tp *standardTopPushdown) GetMaxValue() int {
	return tp.MaxValue
}

// standardCountPushdown implements CountPushdown
type standardCountPushdown struct {
	Dialect     string `json:"dialect,omitempty" yaml:"dialect,omitempty"`
	ParamName   string `json:"paramName,omitempty" yaml:"paramName,omitempty"`
	ParamValue  string `json:"paramValue,omitempty" yaml:"paramValue,omitempty"`
	ResponseKey string `json:"responseKey,omitempty" yaml:"responseKey,omitempty"`
}

func (cp *standardCountPushdown) GetDialect() string {
	if cp.Dialect == "" {
		return CustomDialect
	}
	return cp.Dialect
}

func (cp *standardCountPushdown) GetParamName() string {
	if cp.ParamName == "" && cp.GetDialect() == ODataDialect {
		return DefaultCountParamName
	}
	return cp.ParamName
}

func (cp *standardCountPushdown) GetParamValue() string {
	if cp.ParamValue == "" && cp.GetDialect() == ODataDialect {
		return DefaultCountParamValue
	}
	return cp.ParamValue
}

func (cp *standardCountPushdown) GetResponseKey() string {
	if cp.ResponseKey == "" && cp.GetDialect() == ODataDialect {
		return DefaultCountResponseKey
	}
	return cp.ResponseKey
}

// isItemSupported checks if an item is in the supported list
// Returns true if: list is nil/empty (all supported), list contains "*", or list contains the item
func isItemSupported(item string, supportedList []string) bool {
	if len(supportedList) == 0 {
		return true // empty/nil means all supported
	}
	for _, s := range supportedList {
		if s == "*" || s == item {
			return true
		}
	}
	return false
}

// GetTestingQueryParamPushdown returns a zero-value struct for testing
func GetTestingQueryParamPushdown() standardQueryParamPushdown {
	return standardQueryParamPushdown{}
}
