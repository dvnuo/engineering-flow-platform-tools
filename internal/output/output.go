package output

import (
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type ErrorDetail struct {
	Code    string `json:"code" yaml:"code"`
	Message string `json:"message" yaml:"message"`
	Hint    string `json:"hint,omitempty" yaml:"hint,omitempty"`
}

type Envelope struct {
	OK       bool         `json:"ok" yaml:"ok"`
	Instance string       `json:"instance,omitempty" yaml:"instance,omitempty"`
	Data     interface{}  `json:"data,omitempty" yaml:"data,omitempty"`
	Error    *ErrorDetail `json:"error,omitempty" yaml:"error,omitempty"`
}

func Success(instance string, data interface{}) Envelope {
	return Envelope{OK: true, Instance: instance, Data: data}
}

func Failure(code, message, hint string) Envelope {
	return Envelope{OK: false, Error: &ErrorDetail{Code: code, Message: message, Hint: hint}}
}

func Print(w io.Writer, format string, env Envelope) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(env)
	case "yaml":
		b, err := yaml.Marshal(env)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(b))
		return err
	case "table":
		if env.OK {
			_, err := fmt.Fprintf(w, "ok=true instance=%s\n", env.Instance)
			return err
		}
		_, err := fmt.Fprintf(w, "ok=false code=%s message=%s\n", env.Error.Code, env.Error.Message)
		return err
	default:
		return fmt.Errorf("unknown_output_format")
	}
}
