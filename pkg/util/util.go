package util

import (
	"io/ioutil"
	"os"
)

func EnsureDir(dirName string) error {
	err := os.MkdirAll(dirName, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func TempFileName(dir, prefix string) string {
	file, err := ioutil.TempFile(dir, prefix)
	if err != nil {
		return ""
	}
	return file.Name()
}
