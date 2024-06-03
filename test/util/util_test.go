package util_test

import (
    "controller/pkg/util"
    "testing"
    "os"

    "github.com/stretchr/testify/assert"
)

func TestEnsureDir(t *testing.T) {
    dir := util.TempFileName("", "testdir")
    err := util.EnsureDir(dir)
    assert.NoError(t, err)
    defer os.RemoveAll(dir)

    info, err := os.Stat(dir)
    assert.NoError(t, err)
    assert.True(t, info.IsDir())
}

func TestTempFileName(t *testing.T) {
    fileName := util.TempFileName("", "testfile")
    defer os.Remove(fileName)

    info, err := os.Stat(fileName)
    assert.NoError(t, err)
    assert.False(t, info.IsDir())
}
