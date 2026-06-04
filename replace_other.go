//go:build !windows

package main

import "os"

func replaceFileAtomically(srcPath, destPath string) error {
	return os.Rename(srcPath, destPath)
}
