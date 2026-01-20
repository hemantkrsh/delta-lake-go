package delta

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestPut_OverwriteBehavior(t *testing.T) {
	fixture := setupStorageTest(t)
	defer fixture.cleanup()

	key := "test/file.txt"
	initialContent := []byte("initial content")
	newContent := []byte("new content")

	// First write - should succeed
	if err := fixture.storage.put(key, initialContent); err != nil {
		t.Fatalf("First write failed: %v", err)
	}

	// Verify first write
	fixture.verifyContent(t, filepath.Join(fixture.tempDir, key), initialContent)

	// Second write - should overwrite
	if err := fixture.storage.put(key, newContent); err != nil {
		t.Fatalf("Second write (overwrite) failed: %v", err)
	}

	// Verify second write
	fixture.verifyContent(t, filepath.Join(fixture.tempDir, key), newContent)
}

func TestPutObject_BasicOperations(t *testing.T) {
	fixture := setupStorageTest(t)
	defer fixture.cleanup()

	key := "test/object.data"
	data := []byte("test data")

	t.Run("create new object", func(t *testing.T) {
		if err := fixture.storage.putObject(key, data); err != nil {
			t.Fatalf("Failed to create object: %v", err)
		}
		fixture.verifyContent(t, filepath.Join(fixture.tempDir, key), data)
	})

	t.Run("fail on existing object", func(t *testing.T) {
		err := fixture.storage.putObject(key, []byte("new data"))
		if err == nil {
			t.Fatal("Expected error when writing to existing object, got nil")
		}
		// Verify original content is preserved
		fixture.verifyContent(t, filepath.Join(fixture.tempDir, key), data)
	})
}

func TestPutIfAbsent_KeyBehavior(t *testing.T) {
	fixture := setupStorageTest(t)
	defer fixture.cleanup()

	key := "test/unique.key"
	initialData := []byte("first write")
	secondData := []byte("should not be written")

	t.Run("write to non-existent key", func(t *testing.T) {
		err := fixture.storage.putIfAbsent(key, initialData)
		if err != nil {
			t.Fatalf("First write failed: %v", err)
		}
		fixture.verifyContent(t, filepath.Join(fixture.tempDir, key), initialData)
	})

	t.Run("skip write for existing key", func(t *testing.T) {
		err := fixture.storage.putIfAbsent(key, secondData)
		if err == nil {
			t.Error("Expected error when writing to existing key, got nil")
		}
		// Verify content is still from first write
		fixture.verifyContent(t, filepath.Join(fixture.tempDir, key), initialData)
	})
}

func TestReadRows(t *testing.T) {
	fixture := setupStorageTest(t)
	defer fixture.cleanup()

	// Test data
	testData := dataObject{
		Data: [][]any{
			{1, "test1"},
			{2, "test2"},
		},
	}

	// Convert test data to JSON
	jsonData, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// Create test file
	testFile := "delta/table1/test.data"
	if err := fixture.storage.put(testFile, jsonData); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test reading the file
	data, err := fixture.storage.read(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Unmarshal and verify data
	var result dataObject
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal test data: %v", err)
	}

	// Verify data structure
	if len(result.Data) != 2 {
		t.Fatalf("Expected 2 data items, got %d", len(result.Data))
	}

	// Verify first item
	if result.Data[0][0] != 1.0 || result.Data[0][1] != "test1" {
		t.Errorf("Unexpected data in first item: %+v", result.Data[0])
	}

	// Verify second item
	if result.Data[1][0] != 2.0 || result.Data[1][1] != "test2" {
		t.Errorf("Unexpected data in second item: %+v", result.Data[1])
	}
}

func TestListPrefix_FileListing(t *testing.T) {
	fixture := setupStorageTest(t)
	defer fixture.cleanup()

	// Setup test files
	testFiles := map[string][]byte{
		"delta/logs/0001.json":                       []byte("log1"),
		"delta/logs/0002.json":                       []byte("log2"),
		"delta/checkpoints/0001.checkpoint.parquet":  []byte("check1"),
		"delta/checkpoints/0002.checkpoint.parquet":  []byte("check2"),
		"delta/_delta_log/00000000000000000000.json": []byte("delta_log"),
	}

	if err := fixture.createTestFiles(testFiles); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}

	tests := []struct {
		name     string
		prefix   string
		expected []string
	}{
		{
			name:     "list all delta logs",
			prefix:   "delta/logs/",
			expected: []string{"0001.json", "0002.json"},
		},
		{
			name:     "list checkpoints",
			prefix:   "delta/checkpoints/",
			expected: []string{"0001.checkpoint.parquet", "0002.checkpoint.parquet"},
		},
		{
			name:     "non-existent prefix",
			prefix:   "non/existent/",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := fixture.storage.listPrefix(tt.prefix, "")
			if err != nil {
				t.Fatalf("listPrefix failed: %v", err)
			}
			if len(files) != len(tt.expected) {
				t.Fatalf("Expected %d files, got %d", len(tt.expected), len(files))
			}
			for i, f := range files {
				if f != tt.expected[i] {
					t.Errorf("Mismatch at index %d: got %q, want %q", i, f, tt.expected[i])
				}
			}
		})
	}
}

func TestListPrefixWithStartAfter(t *testing.T) {
	fixture := setupStorageTest(t)
	defer fixture.cleanup()

	// Setup test files
	testFiles := map[string][]byte{
		"0000.log":             []byte("log0"),
		"0001.log":             []byte("log1"),
		"0002.checkpoint.json": []byte("check2"),
		"0003.log":             []byte("log3"),
		"0004.checkpoint.json": []byte("check4"),
		"0005.log":             []byte("log5"),
	}

	if err := fixture.createTestFiles(testFiles); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}

	tests := []struct {
		name       string
		prefix     string
		startAfter string
		want       []string
		wantErr    bool
	}{
		{
			name:       "start after 0002.checkpoint",
			prefix:     fixture.tempDir,
			startAfter: "0002.checkpoint.json",
			want:       []string{"0003.log", "0004.checkpoint.json", "0005.log"},
			wantErr:    false,
		},
		{
			name:       "start after non-existent file",
			prefix:     fixture.tempDir,
			startAfter: "9999.log",
			want:       []string{},
			wantErr:    false,
		},
		{
			name:       "empty startAfter",
			prefix:     fixture.tempDir,
			startAfter: "",
			want:       []string{"0000.log", "0001.log", "0002.checkpoint.json", "0003.log", "0004.checkpoint.json", "0005.log"},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Override the baseDir for this test
			storage := NewFileObjectStorage("")
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
						t.Errorf("Mismatch at index %d: got %q, want %q", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestRead_FileOperations(t *testing.T) {
	fixture := setupStorageTest(t)
	defer fixture.cleanup()

	// Setup test files
	testFiles := map[string][]byte{
		"test/read.txt":  []byte("test content"),
		"test/empty.txt": []byte{},
	}

	if err := fixture.createTestFiles(testFiles); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}

	testCases := []struct {
		name        string
		key         string
		content     []byte
		expectError bool
	}{
		{
			name:        "read existing file",
			key:         "test/read.txt",
			content:     []byte("test content"),
			expectError: false,
		},
		{
			name:        "read non-existent file",
			key:         "non/existent.txt",
			content:     nil,
			expectError: true,
		},
		{
			name:        "read empty file",
			key:         "test/empty.txt",
			content:     []byte{},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test
			data, err := fixture.storage.read(tc.key)

			// Verify error
			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify content
			if string(data) != string(tc.content) {
				t.Errorf("Content mismatch.\nGot: %q\nWant: %q", string(data), string(tc.content))
			}
		})
	}
}

func TestKeyExists_FileOperations(t *testing.T) {
	fixture := setupStorageTest(t)
	defer fixture.cleanup()

	// Setup test files
	testFiles := map[string][]byte{
		"test/exists.txt":     []byte("test content"),
		"test/empty_file.txt": {},
	}

	if err := fixture.createTestFiles(testFiles); err != nil {
		t.Fatalf("Failed to create test files: %v", err)
	}

	testCases := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "existing file",
			key:      "test/exists.txt",
			expected: true,
		},
		{
			name:     "non-existent file",
			key:      "test/not_exists.txt",
			expected: false,
		},
		{
			name:     "empty file",
			key:      "test/empty_file.txt",
			expected: true,
		},
		{
			name:     "non-existent path",
			key:      "non/existent/file.txt",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exists, err := fixture.storage.keyExists(tc.key)
			if err != nil {
				t.Fatalf("keyExists() returned error: %v", err)
			}
			if exists != tc.expected {
				t.Errorf("keyExists() = %v, want %v for key %q", exists, tc.expected, tc.key)
			}
		})
	}
}
