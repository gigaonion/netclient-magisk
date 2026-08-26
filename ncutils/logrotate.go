package ncutils

import (
	"fmt"
	"io"
	"os"
)

const (
	// DefaultLogPath is the main log file path
	DefaultLogPath = "/data/adb/netclient/netclient.log"
	// DefaultMaxLogSize is the threshold before rotating (2 MB)
	DefaultMaxLogSize int64 = 2 * 1024 * 1024
	// DefaultMaxLogBackups is the number of rotated logs to keep
	DefaultMaxLogBackups = 2
)

// RotateLogFile checks if the log file exceeds maxBytes and rotates it using copytruncate.
func RotateLogFile(logPath string, maxBytes int64, maxBackups int) error {
	if logPath == "" {
		logPath = DefaultLogPath
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxLogSize
	}
	if maxBackups <= 0 {
		maxBackups = DefaultMaxLogBackups
	}

	fi, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if fi.Size() < maxBytes {
		return nil
	}

	// Rotate existing backups: .1 -> .2, etc.
	for i := maxBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", logPath, i)
		dst := fmt.Sprintf("%s.%d", logPath, i+1)
		_ = os.Rename(src, dst)
	}

	// Copy current log to .1
	backupPath := fmt.Sprintf("%s.1", logPath)
	srcFile, err := os.Open(logPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Truncate original file in-place so open file descriptors stay valid at offset 0
	_ = os.Truncate(logPath, 0)
	return nil
}
