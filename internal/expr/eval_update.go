package expr

import (
	"fmt"
	"strconv"

	dynago "github.com/danielmensah/dynago"
)

// EvalUpdate applies a list of update expression nodes to an item.
// It returns a new item with the updates applied; the input is not mutated.
func EvalUpdate(nodes []Node, item map[string]dynago.AttributeValue) (map[string]dynago.AttributeValue, error) {
	// Deep copy the item.
	result := copyItem(item)

	for _, node := range nodes {
		un, ok := node.(UpdateNode)
		if !ok {
			return nil, fmt.Errorf("eval_update: expected UpdateNode, got %T", node)
		}
		var err error
		switch un.Action {
		case SET:
			err = applySet(un, result)
		case ADD:
			err = applyAdd(un, result)
		case REMOVE:
			err = applyRemove(un, result)
		case DELETE:
			err = applyDelete(un, result)
		default:
			err = fmt.Errorf("eval_update: unknown action %d", un.Action)
		}
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// copyItem creates a shallow copy of the top-level map. For nested maps, the
// copy is made on write by the set helpers.
func copyItem(item map[string]dynago.AttributeValue) map[string]dynago.AttributeValue {
	out := make(map[string]dynago.AttributeValue, len(item))
	for k, v := range item {
		out[k] = v
	}
	return out
}

// copyMap creates a shallow copy of a map attribute.
func copyMap(m map[string]dynago.AttributeValue) map[string]dynago.AttributeValue {
	out := make(map[string]dynago.AttributeValue, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// setNestedPath sets a value at a potentially nested path, creating
// intermediate maps as needed.
func setNestedPath(parts []string, item map[string]dynago.AttributeValue, val dynago.AttributeValue) error {
	if len(parts) == 1 {
		item[parts[0]] = val
		return nil
	}
	// Navigate/create intermediate maps.
	cur, ok := item[parts[0]]
	if !ok {
		// Create intermediate map.
		inner := make(map[string]dynago.AttributeValue)
		if err := setNestedPath(parts[1:], inner, val); err != nil {
			return err
		}
		item[parts[0]] = dynago.AttributeValue{Type: dynago.TypeM, M: inner}
		return nil
	}
	if cur.Type != dynago.TypeM {
		return fmt.Errorf("eval_update: path part %q is not a map", parts[0])
	}
	// Copy on write for nested maps.
	newMap := copyMap(cur.M)
	if err := setNestedPath(parts[1:], newMap, val); err != nil {
		return err
	}
	item[parts[0]] = dynago.AttributeValue{Type: dynago.TypeM, M: newMap}
	return nil
}

// removeNestedPath removes a value at a potentially nested path.
func removeNestedPath(parts []string, item map[string]dynago.AttributeValue) {
	if len(parts) == 1 {
		delete(item, parts[0])
		return
	}
	cur, ok := item[parts[0]]
	if !ok || cur.Type != dynago.TypeM {
		return
	}
	newMap := copyMap(cur.M)
	removeNestedPath(parts[1:], newMap)
	item[parts[0]] = dynago.AttributeValue{Type: dynago.TypeM, M: newMap}
}

func applySet(un UpdateNode, item map[string]dynago.AttributeValue) error {
	if un.Value == nil {
		return fmt.Errorf("eval_update: SET requires a value")
	}
	val, ok, err := resolveNode(un.Value, item)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("eval_update: SET value could not be resolved")
	}
	return setNestedPath(un.Path.Parts, item, val)
}

func applyAdd(un UpdateNode, item map[string]dynago.AttributeValue) error {
	if un.Value == nil {
		return fmt.Errorf("eval_update: ADD requires a value")
	}
	val, ok, err := resolveNode(un.Value, item)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("eval_update: ADD value could not be resolved")
	}

	existing, exists := resolvePathValue(un.Path.Parts, item)

	if !exists {
		// If path doesn't exist, SET the value (works for numbers and sets).
		return setNestedPath(un.Path.Parts, item, val)
	}

	// ADD on a number: add the values.
	if existing.Type == dynago.TypeN && val.Type == dynago.TypeN {
		ef, err := strconv.ParseFloat(existing.N, 64)
		if err != nil {
			return fmt.Errorf("eval_update: invalid number %q: %w", existing.N, err)
		}
		vf, err := strconv.ParseFloat(val.N, 64)
		if err != nil {
			return fmt.Errorf("eval_update: invalid number %q: %w", val.N, err)
		}
		result := dynago.AttributeValue{
			Type: dynago.TypeN,
			N:    strconv.FormatFloat(ef+vf, 'f', -1, 64),
		}
		return setNestedPath(un.Path.Parts, item, result)
	}

	// ADD on a string set.
	if existing.Type == dynago.TypeSS && val.Type == dynago.TypeSS {
		merged := addToStringSet(existing.SS, val.SS)
		return setNestedPath(un.Path.Parts, item, dynago.AttributeValue{Type: dynago.TypeSS, SS: merged})
	}

	// ADD on a number set.
	if existing.Type == dynago.TypeNS && val.Type == dynago.TypeNS {
		merged := addToStringSet(existing.NS, val.NS)
		return setNestedPath(un.Path.Parts, item, dynago.AttributeValue{Type: dynago.TypeNS, NS: merged})
	}

	// ADD on a binary set.
	if existing.Type == dynago.TypeBS && val.Type == dynago.TypeBS {
		merged := addToBinarySet(existing.BS, val.BS)
		return setNestedPath(un.Path.Parts, item, dynago.AttributeValue{Type: dynago.TypeBS, BS: merged})
	}

	return fmt.Errorf("eval_update: ADD not supported for type %d with value type %d", existing.Type, val.Type)
}

func applyRemove(un UpdateNode, item map[string]dynago.AttributeValue) error {
	removeNestedPath(un.Path.Parts, item)
	return nil
}

func applyDelete(un UpdateNode, item map[string]dynago.AttributeValue) error {
	if un.Value == nil {
		return fmt.Errorf("eval_update: DELETE requires a value")
	}
	val, ok, err := resolveNode(un.Value, item)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("eval_update: DELETE value could not be resolved")
	}

	existing, exists := resolvePathValue(un.Path.Parts, item)
	if !exists {
		return nil // nothing to delete from
	}

	// DELETE from string set.
	if existing.Type == dynago.TypeSS && val.Type == dynago.TypeSS {
		result := removeFromStringSet(existing.SS, val.SS)
		return setNestedPath(un.Path.Parts, item, dynago.AttributeValue{Type: dynago.TypeSS, SS: result})
	}

	// DELETE from number set.
	if existing.Type == dynago.TypeNS && val.Type == dynago.TypeNS {
		result := removeFromStringSet(existing.NS, val.NS)
		return setNestedPath(un.Path.Parts, item, dynago.AttributeValue{Type: dynago.TypeNS, NS: result})
	}

	// DELETE from binary set.
	if existing.Type == dynago.TypeBS && val.Type == dynago.TypeBS {
		result := removeFromBinarySet(existing.BS, val.BS)
		return setNestedPath(un.Path.Parts, item, dynago.AttributeValue{Type: dynago.TypeBS, BS: result})
	}

	return fmt.Errorf("eval_update: DELETE not supported for type %d", existing.Type)
}

// addToStringSet merges new elements into a set, deduplicating.
func addToStringSet(existing, add []string) []string {
	set := make(map[string]struct{}, len(existing))
	for _, s := range existing {
		set[s] = struct{}{}
	}
	result := make([]string, len(existing))
	copy(result, existing)
	for _, s := range add {
		if _, ok := set[s]; !ok {
			result = append(result, s)
			set[s] = struct{}{}
		}
	}
	return result
}

// removeFromStringSet removes elements from a set.
func removeFromStringSet(existing, remove []string) []string {
	toRemove := make(map[string]struct{}, len(remove))
	for _, s := range remove {
		toRemove[s] = struct{}{}
	}
	var result []string
	for _, s := range existing {
		if _, ok := toRemove[s]; !ok {
			result = append(result, s)
		}
	}
	return result
}

// addToBinarySet merges new elements into a binary set.
func addToBinarySet(existing, add [][]byte) [][]byte {
	result := make([][]byte, len(existing))
	copy(result, existing)
	for _, b := range add {
		found := false
		for _, e := range existing {
			if bytesEqual(e, b) {
				found = true
				break
			}
		}
		if !found {
			result = append(result, b)
		}
	}
	return result
}

// removeFromBinarySet removes elements from a binary set.
func removeFromBinarySet(existing, remove [][]byte) [][]byte {
	var result [][]byte
	for _, e := range existing {
		shouldRemove := false
		for _, r := range remove {
			if bytesEqual(e, r) {
				shouldRemove = true
				break
			}
		}
		if !shouldRemove {
			result = append(result, e)
		}
	}
	return result
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
