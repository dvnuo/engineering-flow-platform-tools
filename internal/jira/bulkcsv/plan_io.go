package bulkcsv

import (
	"encoding/json"
	"os"
)

func WritePrettyJSON(path string, value interface{}) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

func LoadMappingPlan(path string) (MappingPlan, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return MappingPlan{}, InvalidArgs("failed to read mapping plan: %s", err)
	}
	var plan MappingPlan
	if err := json.Unmarshal(b, &plan); err != nil {
		return MappingPlan{}, InvalidArgs("invalid mapping plan JSON: %s", err)
	}
	if plan.Version != PlanVersion || plan.Mode != PlanMode {
		return MappingPlan{}, InvalidArgs("mapping plan version or mode is unsupported")
	}
	return plan, nil
}

func LoadJSONObject(path string) (map[string]interface{}, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, InvalidArgs("failed to read JSON file: %s", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, InvalidArgs("invalid JSON file: %s", err)
	}
	return out, nil
}

func LoadJSONValue(path string) (interface{}, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, InvalidArgs("failed to read JSON file: %s", err)
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, InvalidArgs("invalid JSON file: %s", err)
	}
	return out, nil
}
