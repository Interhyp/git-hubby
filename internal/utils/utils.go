package utils

func WithDefaultAsPtr[T any](value *T, defaultValue T) *T {
	if value == nil {
		return &defaultValue
	}
	return value
}

func WithDefault[T any](in *T, defaultValue T) T {
	if in == nil {
		return defaultValue
	}
	return *in
}

func WithEmptyDefault[T any](value []T) []T {
	if value == nil {
		return make([]T, 0)
	}
	return value
}
