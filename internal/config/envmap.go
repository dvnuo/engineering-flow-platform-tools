package config

import (
	"os"
	"reflect"
	"strconv"
	"strings"
)

// LoadFromEnv decodes the shared EFP config from flat, EFP_-prefixed environment
// variables using the json tags of RootConfig. The env var name for any scalar
// is the literal prefix "EFP_" plus the json-tag path from the root joined with
// "_", uppercased, with every "-" replaced by "_"; slice elements insert their
// 0-based index as its own path segment (e.g. EFP_JIRA_INSTANCES_0_AUTH_TOKEN).
// The EFP_ prefix keeps these names out of other tools' namespaces (AWS_*,
// JIRA_*, JENKINS_*). Only present, non-empty values are applied. It returns
// (cfg, managed) where managed is true iff at least one recognized env var was
// found and applied. Unsupported kinds (maps, etc.) are skipped without error.
// c.Normalize() is applied before returning, matching Load().
func LoadFromEnv(lookup func(string) (string, bool)) (RootConfig, bool) {
	var c RootConfig
	count := decodeStruct(reflect.ValueOf(&c).Elem(), nil, lookup)
	c.Normalize()
	return c, count > 0
}

// LoadFromOSEnv is LoadFromEnv backed by the process environment.
func LoadFromOSEnv() (RootConfig, bool) {
	return LoadFromEnv(os.LookupEnv)
}

// envName builds the environment variable name from a slice of json-tag path
// segments: the literal "EFP_" prefix plus the segments joined with "_", with
// "-" replaced by "_", uppercased.
func envName(segs []string) string {
	return "EFP_" + strings.ToUpper(strings.ReplaceAll(strings.Join(segs, "_"), "-", "_"))
}

// jsonName returns the json tag name of a struct field (without options such as
// ",omitempty"). Returns "" when the field has no json tag.
func jsonName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return ""
	}
	if idx := strings.IndexByte(tag, ','); idx >= 0 {
		return tag[:idx]
	}
	return tag
}

// appendSeg returns a fresh slice with seg appended, so recursive calls never
// alias a shared backing array.
func appendSeg(path []string, seg string) []string {
	out := make([]string, len(path)+1)
	copy(out, path)
	out[len(path)] = seg
	return out
}

// decodeStruct recurses over the exported, json-tagged fields of a struct value
// and returns the number of env vars applied.
func decodeStruct(v reflect.Value, path []string, lookup func(string) (string, bool)) int {
	t := v.Type()
	count := 0
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}
		name := jsonName(f)
		if name == "" || name == "-" {
			continue
		}
		count += decodeField(v.Field(i), appendSeg(path, name), lookup)
	}
	return count
}

// decodeField decodes a single field by kind and returns the number of env vars
// applied. Unsupported kinds are skipped and contribute zero.
func decodeField(v reflect.Value, path []string, lookup func(string) (string, bool)) int {
	switch v.Kind() {
	case reflect.Struct:
		return decodeStruct(v, path, lookup)
	case reflect.Slice:
		switch v.Type().Elem().Kind() {
		case reflect.Struct:
			return decodeStructSlice(v, path, lookup)
		case reflect.String:
			return decodeStringSlice(v, path, lookup)
		default:
			return 0
		}
	case reflect.Ptr:
		elemType := v.Type().Elem()
		if elemType.Kind() == reflect.Struct {
			ptr := reflect.New(elemType)
			c := decodeStruct(ptr.Elem(), path, lookup)
			if c > 0 {
				v.Set(ptr)
			}
			return c
		}
		raw, ok := present(lookup, envName(path))
		if !ok {
			return 0
		}
		ptr := reflect.New(elemType)
		if !setScalar(ptr.Elem(), raw) {
			return 0
		}
		v.Set(ptr)
		return 1
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		raw, ok := present(lookup, envName(path))
		if !ok {
			return 0
		}
		if setScalar(v, raw) {
			return 1
		}
		return 0
	default:
		// map and any other unsupported kind: skip without error.
		return 0
	}
}

// decodeStructSlice probes indices 0..N, decoding an element at each PATH_<i>_
// prefix, stopping at the first index whose element consumes zero env vars.
func decodeStructSlice(v reflect.Value, path []string, lookup func(string) (string, bool)) int {
	elemType := v.Type().Elem()
	total := 0
	var elems []reflect.Value
	for i := 0; i < 1024; i++ {
		elem := reflect.New(elemType).Elem()
		c := decodeStruct(elem, appendSeg(path, strconv.Itoa(i)), lookup)
		if c == 0 {
			break
		}
		total += c
		elems = append(elems, elem)
	}
	if len(elems) > 0 {
		s := reflect.MakeSlice(v.Type(), len(elems), len(elems))
		for i, e := range elems {
			s.Index(i).Set(e)
		}
		v.Set(s)
	}
	return total
}

// decodeStringSlice probes PATH_<i> until a value is absent.
func decodeStringSlice(v reflect.Value, path []string, lookup func(string) (string, bool)) int {
	total := 0
	var vals []string
	for i := 0; i < 1024; i++ {
		raw, ok := present(lookup, envName(appendSeg(path, strconv.Itoa(i))))
		if !ok {
			break
		}
		vals = append(vals, raw)
		total++
	}
	if len(vals) > 0 {
		s := reflect.MakeSlice(v.Type(), len(vals), len(vals))
		for i, val := range vals {
			s.Index(i).SetString(val)
		}
		v.Set(s)
	}
	return total
}

// present looks up name and treats an empty value as absent, matching native's
// "only present, non-empty values are emitted" convention.
func present(lookup func(string) (string, bool), name string) (string, bool) {
	raw, ok := lookup(name)
	if !ok || raw == "" {
		return "", false
	}
	return raw, true
}

// setScalar assigns raw into a scalar reflect.Value. It returns false on parse
// errors or unsupported kinds so the caller can treat the field as not-set.
func setScalar(v reflect.Value, raw string) bool {
	switch v.Kind() {
	case reflect.String:
		v.SetString(raw)
		return true
	case reflect.Bool:
		b, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return false
		}
		v.SetBool(b)
		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return false
		}
		v.SetInt(n)
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return false
		}
		v.SetUint(n)
		return true
	default:
		return false
	}
}
