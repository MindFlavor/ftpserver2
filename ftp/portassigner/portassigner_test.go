package portassigner

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreation(t *testing.T) {
	pa := New(10000, 11000)

	assert.NotNil(t, pa)

	pa.Close()
}

func TestClose(t *testing.T) {
	pa := New(10000, 11000)

	assert.NotNil(t, pa)

	pa.Close()

	assert.Panics(t, func() { pa.AssignPort() })
	assert.Panics(t, func() { pa.ReleasePort(0) })
	assert.Panics(t, func() { pa.Close() })
}

func TestAssign(t *testing.T) {
	pa := New(10000, 11000)
	assert.NotNil(t, pa)

	port, err := pa.AssignPort()

	assert.NoError(t, err)
	assert.Equal(t, 10000, port)

	port, err = pa.AssignPort()
	assert.NoError(t, err)
	assert.Equal(t, 10001, port)

	port, err = pa.AssignPort()
	assert.NoError(t, err)
	assert.Equal(t, 10002, port)

	pa.Close()
}

func TestAssignAndRelease(t *testing.T) {
	pa := New(10000, 11000)
	assert.NotNil(t, pa)

	port, err := pa.AssignPort()

	assert.NoError(t, err)
	assert.Equal(t, 10000, port)

	port, err = pa.AssignPort()
	assert.NoError(t, err)
	assert.Equal(t, 10001, port)

	pa.ReleasePort(10000)

	port, err = pa.AssignPort()
	assert.NoError(t, err)
	assert.Equal(t, 10000, port)

	pa.Close()
}

func TestExausthedPorts(t *testing.T) {
	pa := New(10000, 10002)
	assert.NotNil(t, pa)

	port, err := pa.AssignPort()
	assert.NoError(t, err)
	assert.Equal(t, 10000, port)

	port, err = pa.AssignPort()
	assert.NoError(t, err)
	assert.Equal(t, 10001, port)

	port, err = pa.AssignPort()
	assert.Error(t, err)
}
