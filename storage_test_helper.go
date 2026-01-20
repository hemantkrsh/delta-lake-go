package delta

import (
	"os"
	"path/filepath"
	"testing"
)

type testStorageFixture struct {
	tempDir  string
	storage  *fileObjectStorage
	basePath string
}

func setupStorageTest(t *testing.T) *testStorageFixture {
	tempDir := t.TempDir()
	storage := NewFileObjectStorage(tempDir)
	return &testStorageFixture{
		tempDir:  tempDir,
		storage:  &storage,
		basePath: tempDir,
	}
}

func (f *testStorageFixture) createTestFiles(files map[string][]byte) error {
	for file, content := range files {
		path := filepath.Join(f.tempDir, file)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, content, 0644); err != nil {
			return err
		}
	}
	return nil
}

func (f *testStorageFixture) cleanup() {
	os.RemoveAll(f.tempDir)
}

// verifyContent verifies that the file at the given path has the expected content
func (f *testStorageFixture) verifyContent(t *testing.T, path string, expected []byte) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != string(expected) {
		t.Errorf("Content mismatch.\nGot: %q\nWant: %q", content, expected)
	}
}
