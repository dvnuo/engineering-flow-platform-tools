package files

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func ReadBodyFromFlags(body, bodyFile string, bodyStdin bool) (string, error) {
	if body != "" {
		return body, nil
	}
	if bodyFile != "" {
		b, err := os.ReadFile(bodyFile)
		return string(b), err
	}
	if bodyStdin {
		b, err := io.ReadAll(os.Stdin)
		return string(b), err
	}
	return "", nil
}

func ReadJSONValueFromFlags(value, valueFile string) (interface{}, error) {
	var raw []byte
	if value != "" {
		raw = []byte(value)
	} else if valueFile != "" {
		b, err := os.ReadFile(valueFile)
		if err != nil {
			return nil, err
		}
		raw = b
	} else {
		return nil, nil
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("invalid_args: %w", err)
	}
	return out, nil
}
