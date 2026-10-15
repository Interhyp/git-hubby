// Package merge provides helpers to combine desired definitions from multiple sources (e.g. referenced presets and
// inline spec entries) into a single de-duplicated set, detecting conflicting definitions.
package merge

import "fmt"

// InlineSource returns the source description of an inline entry (ruleset, webhook, autolink, ...) at the given index.
func InlineSource(idx int) string {
	return fmt.Sprintf("inline[%d]", idx)
}

// Conflict describes a key that is defined by multiple sources with differing content.
type Conflict struct {
	Key     string
	Sources []string
}

// Dedupe keeps the first candidate per key. Later candidates with equal content are dropped (onDuplicate is called),
// later candidates with different content result in a conflict for that key. Conflicts are returned in the order of
// their first occurrence and list all sources defining the key.
func Dedupe[T any](candidates []T, keyFn func(T) string, equalFn func(a, b T) bool, sourceFn func(T) string,
	onDuplicate func(kept, dup T)) ([]T, []Conflict) {
	type info struct {
		keptIdx  int
		sources  []string
		conflict bool
	}
	result := make([]T, 0, len(candidates))
	byKey := make(map[string]*info, len(candidates))
	conflictOrder := make([]string, 0)

	for _, c := range candidates {
		key := keyFn(c)
		i, seen := byKey[key]
		if !seen {
			byKey[key] = &info{keptIdx: len(result), sources: []string{sourceFn(c)}}
			result = append(result, c)
			continue
		}
		kept := result[i.keptIdx]
		i.sources = append(i.sources, sourceFn(c))
		if equalFn(kept, c) {
			onDuplicate(kept, c)
			continue
		}
		if !i.conflict {
			i.conflict = true
			conflictOrder = append(conflictOrder, key)
		}
	}

	conflicts := make([]Conflict, 0, len(conflictOrder))
	for _, key := range conflictOrder {
		conflicts = append(conflicts, Conflict{Key: key, Sources: byKey[key].sources})
	}
	return result, conflicts
}
