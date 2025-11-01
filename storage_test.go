package delta

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"
)

func TestListPrefix(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get current working directory")
	}
	baseDir := path.Join(cwd, "delta", "table1", logDir)
	t.Logf("Base directory: %s", baseDir)
	dir, err := os.Open(baseDir)
	if err != nil {
		t.Errorf("Failed to open directory")
	}
	defer dir.Close()

	files, err := dir.Readdirnames(500)
	if err != nil && err != io.EOF {
		t.Errorf("Failed to read directory")
	}

	for _, file := range files {
		t.Logf("%s", file)
	}
}

func TestPutIfAbsent(t *testing.T) {
	name := "table1/ty-jkjk.data"
	data := []byte("test")
	cwd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get current working directory")
	}
	storage := NewFileObjectStorage(path.Join(cwd, "test"))
	err = storage.putIfAbsent(name, data)
	if err != nil {
		t.Errorf("Failed to put file: %v", err)
	}

	// verify
	file, err := os.ReadFile(path.Join(cwd, "test", name))
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}
	if string(file) != string(data) {
		t.Errorf("Expected %s, got %s", string(data), string(file))
	}
}

func TestLink(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get current working directory")
	}
	dir1, err := os.MkdirTemp(cwd, "temporary")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(dir1)

	dir2, err := os.MkdirTemp(cwd, "temporary1")
	if err != nil {
		t.Errorf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(dir2)

	file1, err := os.OpenFile(
		path.Join(dir2, "file1"),
		os.O_CREATE|os.O_WRONLY|os.O_EXCL,
		0644,
	)
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}
	file1.Close()

	file2, err := os.OpenFile(
		path.Join(dir2, "file2"),
		os.O_CREATE|os.O_WRONLY|os.O_EXCL,
		0644,
	)
	if err != nil {
		t.Errorf("Failed to create file: %v", err)
	}
	file2.Close()

	file := path.Join(dir1, "file")

	err = os.Link(path.Join(dir2, "file1"), file)
	if err != nil {
		t.Errorf("Failed to link file: %v", err)
	}

	err = os.Link(path.Join(dir2, "file2"), file)
	if err != nil {
		t.Errorf("Failed to link file: %v", err)
	}
}

func TestReadRows(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	file := path.Join(cwd, "delta/table1/2eed6089-6f58-4a7a-82ec-642fedcb7d7c.data")
	bytes, err := os.ReadFile(file)
	if err != nil {
		t.Errorf("error reading file:%v", err)
		return
	}
	var dataObjects dataObject
	err = json.Unmarshal(bytes, &dataObjects)
	if err != nil {
		t.Errorf("error unmarshalling data:%v", err)
		return
	}
	t.Logf("data:%+v", dataObjects.Data[1])
}

func TestListPrefixWithStartAfter(t *testing.T) {
	// Setup test directory structure
	testDir := t.TempDir()
	t.Logf("temp dir: %s", testDir)
	testFiles := []string{
		"0000.log",
		"0001.log",
		"0002.checkpoint.json",
		"0003.log",
		"0004.checkpoint.json",
		"0005.log",
	}

	// Create test files
	for _, f := range testFiles {
		err := os.WriteFile(filepath.Join(testDir, f), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", f, err)
		}
	}

	// Create storage instance
	storage := NewFileObjectStorage("") // baseDir will be overridden in the test

	tests := []struct {
		name       string
		prefix     string
		startAfter string
		want       []string
		wantErr    bool
	}{
		{
			name:       "start after 0002.checkpoint",
			prefix:     testDir,
			startAfter: "0002.checkpoint.json",
			want:       []string{"0003.log", "0004.checkpoint.json", "0005.log"},
			wantErr:    false,
		},
		{
			name:       "start after non-existent file",
			prefix:     testDir,
			startAfter: "9999.log",
			want:       []string{},
			wantErr:    false,
		},
		{
			name:       "empty startAfter",
			prefix:     testDir,
			startAfter: "",
			want:       testFiles,
			wantErr:    false,
		},
		{
			name:       "non-existent directory",
			prefix:     "/non/existent/dir",
			startAfter: "",
			want:       nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Override the baseDir for this test
			storage.deltaBaseDir = filepath.Dir(tt.prefix)
			prefix := filepath.Base(tt.prefix)

			got, err := storage.listPrefix(prefix, tt.startAfter)

			if (err != nil) != tt.wantErr {
				t.Errorf("listPrefix() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("listPrefix() = %v, want %v", got, tt.want)
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("listPrefix() = %v, want %v", got, tt.want)
						break
					}
				}
			}
		})
	}
}
