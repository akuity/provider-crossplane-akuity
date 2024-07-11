package pointer

func ToPointer[T any](value T) *T {
	return &value
}

func ToValue[T any](value *T) T {
	var zero T
	if value == nil {
		return zero
	}
	return *value
}
