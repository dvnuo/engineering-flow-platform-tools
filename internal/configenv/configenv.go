package configenv

import (
	"fmt"
	"reflect"

	"gopkg.in/yaml.v3"
)

// MissingEnvReferenceError reports an environment variable referenced by a
// string value in config.yaml that is unset or empty.
type MissingEnvReferenceError struct {
	Name string
}

func (e *MissingEnvReferenceError) Error() string {
	return fmt.Sprintf("config_env_missing: environment variable %s is not set or is empty", e.Name)
}

// Snapshot records the effective config that was returned to a caller. Stores
// use it as the save baseline so unchanged resolved values can be written back
// as their original references without consulting the current environment.
type Snapshot struct {
	baseline *yaml.Node
	resolved map[string]string
}

// Capture creates a save baseline from the effective, normalized config value.
func Capture(value any, resolution *Resolution) (*Snapshot, error) {
	b, err := yaml.Marshal(value)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return &Snapshot{resolved: resolutionValues(resolution)}, nil
	}
	return &Snapshot{baseline: doc.Content[0], resolved: resolutionValues(resolution)}, nil
}

// Resolution records environment values at load time. It is intentionally
// opaque so callers cannot accidentally log referenced secrets.
type Resolution struct {
	values map[string]string
}

// Resolve replaces exact ${NAME} and %NAME% string values in value. The
// percent form makes Windows-authored config convenient while the braced form
// is portable. Exact matching avoids changing strings that merely contain '$'
// or '%'.
func Resolve(value any, lookup func(string) (string, bool)) (*Resolution, error) {
	resolution := &Resolution{values: map[string]string{}}
	if value == nil {
		return resolution, nil
	}
	if err := resolveValue(reflect.ValueOf(value), lookup, resolution); err != nil {
		return nil, err
	}
	return resolution, nil
}

func resolveValue(value reflect.Value, lookup func(string) (string, bool), resolution *Resolution) error {
	if !value.IsValid() {
		return nil
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Ptr:
		if value.IsNil() {
			return nil
		}
		return resolveValue(value.Elem(), lookup, resolution)
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).PkgPath != "" {
				continue
			}
			if err := resolveValue(value.Field(i), lookup, resolution); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if err := resolveValue(value.Index(i), lookup, resolution); err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			item := reflect.New(value.Type().Elem()).Elem()
			item.Set(iter.Value())
			if err := resolveValue(item, lookup, resolution); err != nil {
				return err
			}
			value.SetMapIndex(iter.Key(), item)
		}
	case reflect.String:
		if !value.CanSet() {
			return nil
		}
		name, ok := ReferenceName(value.String())
		if !ok {
			return nil
		}
		resolved, present := lookup(name)
		if !present || resolved == "" {
			return &MissingEnvReferenceError{Name: name}
		}
		resolution.values[name] = resolved
		value.SetString(resolved)
	}
	return nil
}

// ReferenceName recognizes an exact supported environment reference.
func ReferenceName(value string) (string, bool) {
	var name string
	switch {
	case len(value) > 3 && value[0:2] == "${" && value[len(value)-1] == '}':
		name = value[2 : len(value)-1]
	case len(value) > 2 && value[0] == '%' && value[len(value)-1] == '%':
		name = value[1 : len(value)-1]
	default:
		return "", false
	}
	if !validReferenceName(name) {
		return "", false
	}
	return name, true
}

func validReferenceName(name string) bool {
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

// MergeTopLevel preserves environment references and comments from old while
// applying new. A reference is preserved only when the corresponding effective
// value is unchanged from snapshot. If a loaded config gains an unknown
// reference through a concurrent edit, it is preserved defensively.
func MergeTopLevel(old, new *yaml.Node, snapshot *Snapshot, key string) *yaml.Node {
	var baseline *yaml.Node
	if snapshot != nil {
		baseline = mappingValue(snapshot.baseline, key)
	}
	return mergeNode(old, baseline, new, snapshot)
}

func mergeNode(old, baseline, new *yaml.Node, snapshot *Snapshot) *yaml.Node {
	if old == nil || new == nil {
		return new
	}
	copyComments(old, new)
	if old.Kind == new.Kind && old.Kind == yaml.ScalarNode && old.Style != 0 {
		new.Style = old.Style
	}
	if old.Kind == yaml.ScalarNode && new.Kind == yaml.ScalarNode && old.Tag == "!!str" && new.Tag == "!!str" {
		if _, ok := ReferenceName(old.Value); ok {
			unchanged := baseline != nil && baseline.Kind == yaml.ScalarNode && baseline.Value == new.Value
			if unchanged || (baseline == nil && snapshot != nil) {
				new.Value = old.Value
				new.Style = old.Style
				new.Tag = old.Tag
			}
		}
	}
	switch new.Kind {
	case yaml.MappingNode:
		restoreMovedReferences(old, baseline, new, snapshot)
		oldValues, oldKeys := mappingEntries(old)
		baselineValues, _ := mappingEntries(baseline)
		for i := 0; i+1 < len(new.Content); i += 2 {
			key := new.Content[i].Value
			if oldKey := oldKeys[key]; oldKey != nil {
				copyComments(oldKey, new.Content[i])
			}
			if oldValue := oldValues[key]; oldValue != nil {
				new.Content[i+1] = mergeNode(oldValue, baselineValues[key], new.Content[i+1], snapshot)
			}
		}
	case yaml.SequenceNode:
		oldForNew := matchSequence(old.Content, new.Content)
		oldForBaseline := matchSequence(old.Content, nodeContent(baseline))
		baselineForOld := make(map[int]int, len(oldForBaseline))
		for baselineIndex, oldIndex := range oldForBaseline {
			baselineForOld[oldIndex] = baselineIndex
		}
		for newIndex, oldIndex := range oldForNew {
			var baselineValue *yaml.Node
			if baselineIndex, ok := baselineForOld[oldIndex]; ok && baseline != nil && baselineIndex < len(baseline.Content) {
				baselineValue = baseline.Content[baselineIndex]
			}
			new.Content[newIndex] = mergeNode(old.Content[oldIndex], baselineValue, new.Content[newIndex], snapshot)
		}
	}
	return new
}

// restoreMovedReferences handles normalizers that migrate a resolved value to
// a different key (for example auth.token -> auth.api_key). If exactly one new
// field contains the unchanged load-time value, the original referenced field
// is retained rather than materializing the secret under the migrated key.
func restoreMovedReferences(old, baseline, new *yaml.Node, snapshot *Snapshot) {
	if snapshot == nil || len(snapshot.resolved) == 0 || old == nil || old.Kind != yaml.MappingNode || baseline == nil || baseline.Kind != yaml.MappingNode {
		return
	}
	oldValues, _ := mappingEntries(old)
	baselineValues, _ := mappingEntries(baseline)
	newValues, _ := mappingEntries(new)
	for i := 0; i+1 < len(old.Content); i += 2 {
		oldKey := old.Content[i].Value
		oldValue := old.Content[i+1]
		if newValues[oldKey] != nil || oldValue.Kind != yaml.ScalarNode || oldValue.Tag != "!!str" {
			continue
		}
		name, ok := ReferenceName(oldValue.Value)
		if !ok {
			continue
		}
		resolved, ok := snapshot.resolved[name]
		if !ok {
			continue
		}
		candidate := ""
		for newKey, newValue := range newValues {
			if oldValues[newKey] != nil || newValue.Kind != yaml.ScalarNode || newValue.Tag != "!!str" || newValue.Value != resolved {
				continue
			}
			baselineValue := baselineValues[newKey]
			if baselineValue == nil || baselineValue.Kind != yaml.ScalarNode || baselineValue.Value != newValue.Value {
				continue
			}
			if candidate != "" {
				candidate = ""
				break
			}
			candidate = newKey
		}
		if candidate == "" {
			continue
		}
		removeMappingValue(new, candidate)
		new.Content = append(new.Content, cloneNode(old.Content[i]), cloneNode(oldValue))
		newValues, _ = mappingEntries(new)
	}
}

func removeMappingValue(node *yaml.Node, key string) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return
		}
	}
}

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clone := *node
	clone.Content = make([]*yaml.Node, len(node.Content))
	for i, child := range node.Content {
		clone.Content[i] = cloneNode(child)
	}
	return &clone
}

func resolutionValues(resolution *Resolution) map[string]string {
	if resolution == nil || len(resolution.values) == 0 {
		return nil
	}
	values := make(map[string]string, len(resolution.values))
	for name, value := range resolution.values {
		values[name] = value
	}
	return values
}

func mappingEntries(node *yaml.Node) (map[string]*yaml.Node, map[string]*yaml.Node) {
	values := map[string]*yaml.Node{}
	keys := map[string]*yaml.Node{}
	if node == nil || node.Kind != yaml.MappingNode {
		return values, keys
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		keys[node.Content[i].Value] = node.Content[i]
		values[node.Content[i].Value] = node.Content[i+1]
	}
	return values, keys
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	values, _ := mappingEntries(node)
	return values[key]
}

func nodeContent(node *yaml.Node) []*yaml.Node {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	return node.Content
}

// matchSequence returns right-index -> left-index. Named mapping items are
// paired first, so reordering and removal cannot move references between
// instances. Remaining items pair by index, which also handles a rename while
// exact matches reserve all unaffected entries first.
func matchSequence(left, right []*yaml.Node) map[int]int {
	matches := map[int]int{}
	usedLeft := map[int]bool{}
	leftByName := map[string]int{}
	duplicateName := map[string]bool{}
	for i, node := range left {
		name, ok := sequenceItemName(node)
		if !ok {
			continue
		}
		if _, exists := leftByName[name]; exists {
			duplicateName[name] = true
			continue
		}
		leftByName[name] = i
	}
	for rightIndex, node := range right {
		name, ok := sequenceItemName(node)
		if !ok || duplicateName[name] {
			continue
		}
		leftIndex, exists := leftByName[name]
		if !exists || usedLeft[leftIndex] {
			continue
		}
		matches[rightIndex] = leftIndex
		usedLeft[leftIndex] = true
	}
	for rightIndex := range right {
		if _, ok := matches[rightIndex]; ok || rightIndex >= len(left) || usedLeft[rightIndex] {
			continue
		}
		matches[rightIndex] = rightIndex
		usedLeft[rightIndex] = true
	}
	return matches
}

func sequenceItemName(node *yaml.Node) (string, bool) {
	value := mappingValue(node, "name")
	if value == nil || value.Kind != yaml.ScalarNode || value.Value == "" {
		return "", false
	}
	return value.Value, true
}

func copyComments(from, to *yaml.Node) {
	if from == nil || to == nil {
		return
	}
	if to.HeadComment == "" {
		to.HeadComment = from.HeadComment
	}
	if to.LineComment == "" {
		to.LineComment = from.LineComment
	}
	if to.FootComment == "" {
		to.FootComment = from.FootComment
	}
}
