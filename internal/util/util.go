package util

func MapSlice[T any, U any](slice []T, converter func(T) U) []U {
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = converter(v)
	}
	return result
}

func MapMap[T, U any](m map[string]T, converter func(T) U) map[string]U {
	result := make(map[string]U)
	for k, v := range m {
		res := converter(v)
		result[k] = res
	}
	return result
}
