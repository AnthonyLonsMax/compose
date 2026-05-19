package dbutil

func GroupBy[T any, K comparable](objects []T, keyFunc func(T) K) map[K][]T {
	result := make(map[K][]T)
	for _, object := range objects {
		result[keyFunc(object)] = append(result[keyFunc(object)], object)
	}
	return result
}

func ExtractIDS[T any, KEY any](objects []T, getKeyFunc func(T) KEY) []KEY {
	keys := make([]KEY, 0, len(objects))
	for _, object := range objects {
		keys = append(keys, getKeyFunc(object))
	}
	return keys
}

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
