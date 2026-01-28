package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToPtr(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		val := "hello"
		ptr := ToPtr(val)
		assert.Equal(t, &val, ptr)
	})

	t.Run("int", func(t *testing.T) {
		val := 123
		ptr := ToPtr(val)
		assert.Equal(t, &val, ptr)
	})

	t.Run("struct", func(t *testing.T) {
		type myStruct struct {
			Field string
		}
		val := myStruct{Field: "test"}
		ptr := ToPtr(val)
		assert.Equal(t, &val, ptr)
	})
}