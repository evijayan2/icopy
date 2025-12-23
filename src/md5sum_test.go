package icopy

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestMd5Sum(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test_md5.txt")
	content := []byte("hello world")
	if err := ioutil.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	// Calculate expected MD5
	expectedHash := fmt.Sprintf("%x", md5.Sum(content))

	// Run Md5Sum
	hash, err := Md5Sum(tmpFile)
	if err != nil {
		t.Fatalf("Md5Sum returned error: %v", err)
	}

	if hash != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, hash)
	}
}

func TestMd5Sum_FileNotFound(t *testing.T) {
	_, err := Md5Sum("non_existent_file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}
