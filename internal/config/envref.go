package config

import (
	"fmt"
	"reflect"
)

// MissingEnvReferenceError reports an environment variable referenced by a
// string value in config.yaml that is unset or empty.
type MissingEnvReferenceError struct {
	Name string
}

func (e *MissingEnvReferenceError) Error() string {
	return fmt.Sprintf("config_env_missing: environment variable %s is not set or is empty", e.Name)
}

// resolveEnvReferences replaces exact ${NAME} and %NAME% string values in v.
// The percent form makes Windows-authored config files convenient while the
// braced form is portable across shells. Exact-value matching avoids changing
// literal passwords or URLs that merely contain '$' or '%'.
func resolveEnvReferences(v any, lookup func(string) (string, bool)) error {
	if v == nil {
		return nil
	}
	return resolveEnvValue(reflect.ValueOf(v), lookup)
}

func resolveEnvValue(v reflect.Value, lookup func(string) (string, bool)) error {
	if !v.IsValid() {
		return nil
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		return resolveEnvValue(v.Elem(), lookup)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).PkgPath != "" {
				continue
			}
			if err := resolveEnvValue(v.Field(i), lookup); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := resolveEnvValue(v.Index(i), lookup); err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			value := reflect.New(v.Type().Elem()).Elem()
			value.Set(iter.Value())
			if err := resolveEnvValue(value, lookup); err != nil {
				return err
			}
			v.SetMapIndex(iter.Key(), value)
		}
	case reflect.String:
		if !v.CanSet() {
			return nil
		}
		name, ok := envReferenceName(v.String())
		if !ok {
			return nil
		}
		value, present := lookup(name)
		if !present || value == "" {
			return &MissingEnvReferenceError{Name: name}
		}
		v.SetString(value)
	}
	return nil
}

func envReferenceName(value string) (string, bool) {
	var name string
	switch {
	case len(value) > 3 && value[0:2] == "${" && value[len(value)-1] == '}':
		name = value[2 : len(value)-1]
	case len(value) > 2 && value[0] == '%' && value[len(value)-1] == '%':
		name = value[1 : len(value)-1]
	default:
		return "", false
	}
	if !validEnvReferenceName(name) {
		return "", false
	}
	return name, true
}

func validEnvReferenceName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// preserveEnvReference keeps a placeholder in the persisted YAML when the
// in-memory value is only its resolved form. This prevents unrelated config
// writes from materializing credentials into config.yaml.
func preserveEnvReference(reference, resolved string, lookup func(string) (string, bool)) (string, bool) {
	name, ok := envReferenceName(reference)
	if !ok {
		return "", false
	}
	value, present := lookup(name)
	if !present || value == "" || value != resolved {
		return "", false
	}
	return reference, true
}
