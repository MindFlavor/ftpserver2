package main

import (
	"testing"
	"time"

	"github.com/mindflavor/ftpserver2/ftp"
	"github.com/stretchr/testify/assert"
)

func TestFTPBasic(t *testing.T) {
	// 15 min connection timeout
	timeout, err := time.ParseDuration("15m")

	assert.NoError(t, err)

	ftp := ftp.NewPlain(21, nil, timeout, 5000, 5100, nil, nil)

	assert.NotNil(t, ftp)
}
