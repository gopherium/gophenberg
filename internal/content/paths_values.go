// SPDX-License-Identifier: Apache-2.0

package content

// Stripped returns the values with whatever stands at the key path removed, however deep it runs.
func (v Values) Stripped(path []string) Values {
	if len(path) == 0 {
		return v
	}
	stripped := make(Values, len(v))
	for key, value := range v {
		stripped[key] = value
	}
	if len(path) == 1 {
		delete(stripped, path[0])
		return stripped
	}
	held, found := stripped[path[0]]
	if !found {
		return stripped
	}
	stripped[path[0]] = strippedInside(held, path[1:])
	return stripped
}

// strippedInside returns the value with the rest of the path removed from whatever holds it.
func strippedInside(value any, path []string) any {
	if rows, listed := value.([]any); listed {
		swept := make([]any, len(rows))
		for at, row := range rows {
			swept[at] = strippedInside(row, path)
		}
		return swept
	}
	inside, held := value.(map[string]any)
	if !held {
		return value
	}
	return map[string]any(Values(inside).Stripped(path))
}
