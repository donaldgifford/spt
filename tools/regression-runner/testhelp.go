package main

import "os"

// readFile is a tiny indirection so regression_test.go can read
// neighboring source files without growing the test's import list.
func readFile(name string) (string, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
