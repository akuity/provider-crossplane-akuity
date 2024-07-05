package pointer

func ToPointer[T any](value T) *T {
	return &value
}

func ToValue[T any](value *T, defaultValue T) T {
	if value == nil {
		return defaultValue
	}
	return *value
}

func CopyString(s *string) *string {
	if s == nil {
		return nil
	}

	return ToPointer(ToValue(s, ""))
}
