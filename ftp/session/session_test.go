package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_splitAndClearPathPlain(t *testing.T) {
	received := splitAndClearPath("/root/test/third")
	expected := []string{"root", "test", "third"}
	validate(t, expected, received)
}

func Test_splitAndClearPathLastDoubleDot(t *testing.T) {
	received := splitAndClearPath("/root/test/third/..")
	expected := []string{"root", "test"}
	validate(t, expected, received)
}

func Test_splitAndClearPathSecondDoubleDot(t *testing.T) {
	received := splitAndClearPath("/root/../third")
	expected := []string{"third"}
	validate(t, expected, received)
}

func Test_splitAndClearPathSecondDoubleDotDouble(t *testing.T) {
	received := splitAndClearPath("/root/test/../third/forth/..")
	expected := []string{"root", "third"}
	validate(t, expected, received)
}

func Test_clearPathSecondDoubleDotDouble(t *testing.T) {
	received := clearPath("/root/test/../third/forth/..")
	expected := "/root/third"
	assert.Equal(t, expected, received)
}

func Test_clearPathPlain(t *testing.T) {
	received := clearPath("/root/test/third")
	expected := "/root/test/third"
	assert.Equal(t, expected, received)
}

func Test_clearPathPlainRelative(t *testing.T) {
	received := clearPath("root/test/third")
	expected := "root/test/third"
	assert.Equal(t, expected, received)
}

func Test_clearPathPlainRelativeSecondDoubleDotDouble(t *testing.T) {
	received := clearPath("root/test/../third/forth/..")
	expected := "root/third"
	assert.Equal(t, expected, received)
}

func validate(t *testing.T, expected, received []string) {
	assert.Equal(t, len(expected), len(received))

	for i, exp := range expected {
		assert.Equal(t, exp, received[i])
	}
}
