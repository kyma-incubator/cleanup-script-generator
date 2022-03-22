package main

import (
	"bytes"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCLI(t *testing.T) {
	buf := bytes.NewBufferString("")
	err := run(buf, flags{
		fromFile: path.Join("testdata", "kyma-1.yaml"),
		toFile:   path.Join("testdata", "kyma-2.yaml"),
	})
	require.NoError(t, err)
	require.Empty(t, buf.String())
}
