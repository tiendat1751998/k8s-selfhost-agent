package helm

import (
	"encoding/json"
	"strings"

	"sigs.k8s.io/yaml"
)

// ValuesPayload transparently supports both JSON map objects and JSON strings containing raw YAML.
type ValuesPayload map[string]interface{}

// ToMap returns the underlying map[string]interface{}.
func (v ValuesPayload) ToMap() map[string]interface{} {
	if v == nil {
		return make(map[string]interface{})
	}
	return map[string]interface{}(v)
}

// UnmarshalJSON implements custom JSON unmarshaling supporting raw YAML strings or JSON objects.
func (v *ValuesPayload) UnmarshalJSON(b []byte) error {
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" || trimmed == "null" || trimmed == `""` || trimmed == "{}" {
		*v = make(map[string]interface{})
		return nil
	}

	// Case 1: String-encoded YAML
	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		var yamlStr string
		if err := json.Unmarshal(b, &yamlStr); err != nil {
			return err
		}
		yamlStr = strings.TrimSpace(yamlStr)
		if yamlStr == "" {
			*v = make(map[string]interface{})
			return nil
		}
		var parsed map[string]interface{}
		if err := yaml.Unmarshal([]byte(yamlStr), &parsed); err != nil {
			return err
		}
		if parsed == nil {
			parsed = make(map[string]interface{})
		}
		*v = parsed
		return nil
	}

	// Case 2: Direct JSON object
	var obj map[string]interface{}
	if err := json.Unmarshal(b, &obj); err != nil {
		// Fallback: try raw YAML unmarshal
		if errYaml := yaml.Unmarshal(b, &obj); errYaml == nil && obj != nil {
			*v = obj
			return nil
		}
		return err
	}
	if obj == nil {
		obj = make(map[string]interface{})
	}
	*v = obj
	return nil
}
