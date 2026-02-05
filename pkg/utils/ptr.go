package utils //nolint:revive // var-naming: utils is an acceptable package name for shared utilities

// ToPtr returns a pointer to the given value.
func ToPtr[T any](v T) *T {
	return &v
}
