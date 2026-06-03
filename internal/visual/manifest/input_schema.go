package manifest

import (
	"bytes"
	"encoding/json"
	"io"
	"os"

	"engineering-flow-platform-tools/internal/visual/metadata"
)

const TemplateInputSchemaContract = "efp.visual.template_input_schema.v1"

type TemplateInputSchema struct {
	Schema          string         `json:"schema"`
	TemplateID      string         `json:"template_id"`
	InputSchemaKind string         `json:"input_schema_kind"`
	JSONSchema      map[string]any `json:"json_schema"`
	Example         map[string]any `json:"example"`
}

func LoadTemplateInputSchema(templateDir string, entry RegistryEntry, tpl TemplateManifest) (TemplateInputSchema, string, error) {
	rel, path, err := resolveTemplateInputSchemaPath(templateDir, entry, tpl.InputSchema)
	if err != nil {
		return TemplateInputSchema{}, rel, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return TemplateInputSchema{}, rel, metadata.NewError("schema_not_found", "visual template schema file was not found: "+rel, "Add schema.input.json or update template.yaml input_schema.", 404)
	}
	var doc TemplateInputSchema
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return TemplateInputSchema{}, rel, metadata.NewError("template_manifest_invalid", "visual template schema file is invalid JSON: "+rel+": "+err.Error(), "Fix "+rel+" so it contains a valid template input schema object.", 400)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return TemplateInputSchema{}, rel, metadata.NewError("template_manifest_invalid", "visual template schema file contains extra JSON tokens: "+rel, "Keep one JSON object in "+rel+".", 400)
	}
	if doc.Schema != TemplateInputSchemaContract {
		return TemplateInputSchema{}, rel, metadata.NewError("template_manifest_invalid", "visual template schema file has unsupported schema contract: "+rel, "Set schema to "+TemplateInputSchemaContract+".", 400)
	}
	if doc.TemplateID != tpl.ID {
		return TemplateInputSchema{}, rel, metadata.NewError("template_manifest_invalid", "visual template schema file template_id does not match manifest id: "+rel, "Set template_id to "+tpl.ID+".", 400)
	}
	if doc.InputSchemaKind != tpl.InputSchemaKind {
		return TemplateInputSchema{}, rel, metadata.NewError("template_manifest_invalid", "visual template schema file input_schema_kind does not match manifest: "+rel, "Set input_schema_kind to "+tpl.InputSchemaKind+".", 400)
	}
	if len(doc.JSONSchema) == 0 {
		return TemplateInputSchema{}, rel, metadata.NewError("template_manifest_invalid", "visual template schema file is missing json_schema: "+rel, "Add a complete json_schema object.", 400)
	}
	if len(doc.Example) == 0 {
		return TemplateInputSchema{}, rel, metadata.NewError("template_manifest_invalid", "visual template schema file is missing example: "+rel, "Add an example object matching examples/basic.input.json.", 400)
	}
	return doc, rel, nil
}
