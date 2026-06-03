package render

import (
	"encoding/json"
	"os"

	"engineering-flow-platform-tools/internal/visual/metadata"
)

func writeJSONFile(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return metadata.NewError("output_write_failed", "failed to encode JSON: "+err.Error(), "Inspect the visual input and manifest data.", 500)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return metadata.NewError("output_write_failed", "failed to write "+path+": "+err.Error(), "Check --out permissions.", 500)
	}
	return nil
}

func writeJSAssignment(path, variable string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return metadata.NewError("output_write_failed", "failed to encode JavaScript data: "+err.Error(), "Inspect the visual input and manifest data.", 500)
	}
	out := append([]byte("window."+variable+" = "), b...)
	out = append(out, ';', '\n')
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return metadata.NewError("output_write_failed", "failed to write "+path+": "+err.Error(), "Check --out permissions.", 500)
	}
	return nil
}
