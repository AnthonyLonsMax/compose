// Package eagerload provides generic utilities for building nested data structures
// from relational database queries without the N+1 problem.
//
// The pattern is: query each hierarchy level independently with WHERE IN,
// then use GroupBy + MergeChildren to reconstruct the graph in memory.
// ExtractIDs extracts the foreign keys needed between levels.
//
// For multi-level nesting, query top-down and reconstruct bottom-up:
//
//	parents  := queryLevel1(db)
//	ids      := ExtractIDs(parents, ...)
//	children := queryLevel2(db, ids)
//	...
//	MergeChildren(children, GroupBy(grandchildren, ...), ...)  // bottom level first
//	MergeChildren(parents,   GroupBy(children, ...), ...)      // work up
package eagerload

// GroupBy groups a slice into a map keyed by K using keyFunc.
func GroupBy[T any, K comparable](objects []T, keyFunc func(T) K) map[K][]T {
	result := make(map[K][]T)
	for _, object := range objects {
		result[keyFunc(object)] = append(result[keyFunc(object)], object)
	}

	return result
}

// ExtractIDs extracts a key (typically a database ID) from each element in
// the slice, preserving order. Used to collect foreign keys for WHERE IN queries.
func ExtractIDs[T any, KEY any](objects []T, getKeyFunc func(T) KEY) []KEY {
	keys := make([]KEY, 0, len(objects))
	for _, object := range objects {
		keys = append(keys, getKeyFunc(object))
	}

	return keys
}

// MergeChildren merges grouped children into their parents in-place.
//
// For each parent, parentKeyFunc extracts the lookup key, childMap is searched
// for matching children, and setChildrenFunc writes them into the parent.
//
// TChild is typically a flat row from a query — the setChildrenFunc can
// transform it into the desired child type before storing.
func MergeChildren[TParent, TChild any, K comparable](
	parents []TParent,
	childMap map[K][]TChild,
	parentKeyFunc func(TParent) K,
	setChildrenFunc func(*TParent, []TChild),
) {
	for i := range parents {
		setChildrenFunc(&parents[i], childMap[parentKeyFunc(parents[i])])
	}
}

// Map transforms each element of a slice from type I to type O using mapFunc.
//
// It pre-allocates the output slice with len(elements) capacity but builds
// the result via append, so mapFunc may return zero values to skip elements
// without shifting — the caller can filter afterward if needed.
//
// Typical use: converting database models to DTOs or API responses.
//
// Performance note: the returned slice may have trailing zero values if
// elements are skipped. Use a filtering Map variant or post-filter if this
// matters in hot paths.
func Map[I any, O any](elements []I, mapFunc func(I) O) []O {
	result := make([]O, 0, len(elements))
	for _, element := range elements {
		result = append(result, mapFunc(element))
	}
	return result
}
