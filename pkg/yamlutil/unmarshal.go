// Custom yaml.Unmarshal that decodes yaml objects to map[string]any
// instead of map[any][any].
// See: https://github.com/go-yaml/yaml/issues/139#issuecomment-220072190

package yamlutil

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

// Unmarshal YAML to map[string]interface{} instead of map[interface{}]interface{}.
func Unmarshal(in []byte, out interface{}) error {
	var res interface{}

	if err := yaml.Unmarshal(in, &res); err != nil {
		return err
	}
	*out.(*interface{}) = toMapValue(res)

	return nil
}

// Marshal YAML wrapper function.
func Marshal(in interface{}) ([]byte, error) {
	return yaml.Marshal(in)
}

func AnyArray(in []interface{}) []interface{} {
	res := make([]interface{}, len(in))
	for i, v := range in {
		res[i] = toMapValue(v)
	}
	return res
}

func AnyMap(in map[interface{}]interface{}) map[string]interface{} {
	res := make(map[string]interface{})
	for k, v := range in {
		res[fmt.Sprintf("%v", k)] = toMapValue(v)
	}
	return res
}

func toMapValue(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		return AnyArray(v)
	case map[interface{}]interface{}:
		return AnyMap(v)
	case string, bool, int64, float64, int, float32:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
