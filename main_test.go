package main

import "testing"
import "github.com/stretchr/testify/assert"

func TestFloatToSmallestString(t *testing.T) {
	assert.Equal(t, FloatToSmallestString(1024, 4), "1024")
	assert.Equal(t, FloatToSmallestString(300, 4), "300")

	assert.Equal(t, FloatToSmallestString(12.5, 4), "12.5")
	assert.Equal(t, FloatToSmallestString(12.111111111111, 4), "12.1111")
	assert.Equal(t, FloatToSmallestString(1.499999999999, 4), "1.5")
	assert.Equal(t, FloatToSmallestString(123.12345555555, 4), "123.1235")

	assert.Equal(t, FloatToSmallestString(0.01, 4), ".01")
	assert.Equal(t, FloatToSmallestString(0.001, 4), ".001")
	assert.Equal(t, FloatToSmallestString(0.0001, 4), ".0001")
	assert.Equal(t, FloatToSmallestString(0.00001, 4), "0")
	assert.Equal(t, FloatToSmallestString(0.00005, 4), ".0001")
	assert.Equal(t, FloatToSmallestString(0.00001, 5), ".00001")
}