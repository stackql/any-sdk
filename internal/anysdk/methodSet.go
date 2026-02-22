package anysdk

type MethodSet []StandardOperationStore

func (ms MethodSet) GetFirstMatch(params map[string]interface{}) (StandardOperationStore, map[string]interface{}, bool) {
	return ms.getFirstMatch(params)
}

func (ms MethodSet) GetFirstNamespaceMatch(params map[string]any) (StandardOperationStore, map[string]any, bool) {
	return ms.getFirstNamespaceMatch(params)
}

func (ms MethodSet) GetFirst() (StandardOperationStore, string, bool) {
	return ms.getFirst()
}

func (ms MethodSet) getFirstMatch(params map[string]interface{}) (StandardOperationStore, map[string]interface{}, bool) {
	for _, m := range ms {
		if remainingParams, ok := m.parameterMatch(params); ok {
			return m, remainingParams, true
		}
	}
	return nil, params, false
}

func (ms MethodSet) getFirstNamespaceMatch(params map[string]any) (StandardOperationStore, map[string]any, bool) {
	for _, m := range ms {
		if remainingParams, ok := m.namespaceParameterMatch(params); ok {
			return m, remainingParams, true
		}
	}
	return nil, params, false
}

func (ms MethodSet) getFirst() (StandardOperationStore, string, bool) {
	for _, m := range ms {
		return m, m.getName(), true
	}
	return nil, "", false
}
