package anysdk

import (
	"fmt"
	"math"
	"strconv"
)

// coerceBodyToSchema converts scalar request body values to the JSON type the
// schema declares (e.g. "24" -> 24 for `type: number`). Values with no schema,
// an untyped schema, or a non-scalar mismatch pass through untouched.
func coerceBodyToSchema(s Schema, v interface{}) (interface{}, error) {
	return coerceValueToSchema(s, v, "")
}

func coerceValueToSchema(s Schema, v interface{}, path string) (interface{}, error) {
	if s == nil || v == nil {
		return v, nil
	}
	switch s.GetType() {
	case "object", "":
		return coerceObjectToSchema(s, v, path)
	case "array":
		return coerceArrayToSchema(s, v, path)
	case "integer":
		return coerceToInteger(v, path)
	case "number":
		return coerceToNumber(v, path)
	case "boolean":
		return coerceToBoolean(v, path)
	case "string":
		return coerceToString(v), nil
	}
	return v, nil
}

func coerceObjectToSchema(s Schema, v interface{}, path string) (interface{}, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return v, nil
	}
	rv := make(map[string]interface{}, len(m))
	for k, val := range m {
		ps, hasSchema := s.GetProperty(k)
		if !hasSchema {
			ps, hasSchema = s.GetAdditionalProperties()
		}
		if !hasSchema {
			rv[k] = val
			continue
		}
		cv, err := coerceValueToSchema(ps, val, joinBodyPath(path, k))
		if err != nil {
			return nil, err
		}
		rv[k] = cv
	}
	return rv, nil
}

func coerceArrayToSchema(s Schema, v interface{}, path string) (interface{}, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return v, nil
	}
	items, err := s.GetItems()
	if err != nil {
		return v, nil
	}
	rv := make([]interface{}, len(arr))
	for i, e := range arr {
		cv, cErr := coerceValueToSchema(items, e, fmt.Sprintf("%s[%d]", path, i))
		if cErr != nil {
			return nil, cErr
		}
		rv[i] = cv
	}
	return rv, nil
}

func coerceToInteger(v interface{}, path string) (interface{}, error) {
	switch val := v.(type) {
	case string:
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			return n, nil
		}
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, coercionError(path, val, "integer")
		}
		return integralFloatToInt(f, path)
	case float64:
		return integralFloatToInt(val, path)
	case float32:
		return integralFloatToInt(float64(val), path)
	}
	return v, nil
}

func integralFloatToInt(f float64, path string) (interface{}, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return nil, coercionError(path, f, "integer")
	}
	if f >= math.MaxInt64 || f <= math.MinInt64 {
		return f, nil
	}
	return int64(f), nil
}

func coerceToNumber(v interface{}, path string) (interface{}, error) {
	if val, ok := v.(string); ok {
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return nil, coercionError(path, val, "number")
		}
		return f, nil
	}
	return v, nil
}

func coerceToBoolean(v interface{}, path string) (interface{}, error) {
	if val, ok := v.(string); ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			return nil, coercionError(path, val, "boolean")
		}
		return b, nil
	}
	return v, nil
}

func coerceToString(v interface{}) interface{} {
	switch val := v.(type) {
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case int:
		return strconv.Itoa(val)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case bool:
		return strconv.FormatBool(val)
	}
	return v
}

func coercionError(path string, v interface{}, schemaType string) error {
	return fmt.Errorf("request body key '%s': value '%v' cannot be coerced to schema type '%s'", path, v, schemaType)
}

func joinBodyPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}
