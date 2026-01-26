package delta

import (
	"os"
	"path"
	"strings"
	"testing"
)

// setupTestDir creates a temporary directory for Delta tables
// It returns the path to the temp directory and a cleanup function
func setupTestDir(t *testing.T) (string, func()) {
	t.Helper()

	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "delta-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	t.Logf("temp dir:%s", tmpDir)
	// Create just the base Delta directory
	deltaDir := path.Join(tmpDir, "delta")
	if err := os.MkdirAll(deltaDir, 0755); err != nil {
		t.Fatalf("Failed to create delta directory: %v", err)
	}

	t.Logf("Created test directory at: %s", deltaDir)

	return tmpDir, func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Failed to clean up test directory %s: %v", tmpDir, err)
		}
	}
}

func TestDeltaClientWrite(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()

	storage := NewFileObjectStorage(path.Join(tmpDir, "delta"))
	client := NewDeltaClient(&storage)

	t.Run("Table creation and writes should complete without errors", func(t *testing.T) {
		err := client.nwTxn()
		if err != nil {
			t.Fatalf("Table creation failed at transaction start: %v", err)
		}
		// Create table
		tableName := "test_table"
		schema := []string{"name", "age"}
		if err := client.createTable(tableName, schema); err != nil {
			t.Fatalf("Table creation failed: %v", err)
		}
		if err := client.commit(); err != nil {
			t.Fatalf("Table creation commit failed: %v", err)
		}
		if err := client.nwTxn(); err != nil {
			t.Fatalf("Write transaction start failed: %v", err)
		}
		testData := []struct {
			name string
			age  int
		}{
			{"John", 30},
			{"Jack", 25},
		}
		for i, data := range testData {
			if err := client.writeRow(tableName, []any{data.name, data.age}); err != nil {
				t.Fatalf("Write operation %d failed: %v", i, err)
			}
		}
		// Commit
		if err := client.commit(); err != nil {
			t.Fatalf("Write commit failed: %v", err)
		}
		t.Logf("Successfully completed all operations without errors")
	})
}

func TestDeltaClientRead(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()

	storage := NewFileObjectStorage(path.Join(tmpDir, "delta"))
	client := NewDeltaClient(&storage)
	tableName := "test_read_table"

	testData := []struct {
		name string
		age  int
	}{
		{"John", 30},
		{"Jack", 25},
	}

	t.Run("Write test data", func(t *testing.T) {
		if err := client.nwTxn(); err != nil {
			t.Fatalf("Failed to start transaction: %v", err)
		}

		//Create table
		if err := client.createTable(tableName, []string{"name", "age"}); err != nil {
			t.Fatalf("Failed to create table: %v", err)
		}
		// Write
		for _, data := range testData {
			if err := client.writeRow(tableName, []any{data.name, data.age}); err != nil {
				t.Fatalf("Failed to write row: %v", err)
			}
		}
		// Commit
		if err := client.commit(); err != nil {
			t.Fatalf("Failed to commit writes: %v", err)
		}
	})

	t.Run("Read and verify data", func(t *testing.T) {
		if err := client.nwTxn(); err != nil {
			t.Fatalf("Failed to start read transaction: %v", err)
		}
		// Read
		iterator, err := client.read(tableName)
		if err != nil {
			t.Fatalf("Failed to create iterator: %v", err)
		}

		var rows [][]any
		ok, row := iterator.next()
		for {
			if ok != false || row != nil {
				rows = append(rows, row)
			} else {
				break
			}
			ok, row = iterator.next()
		}

		// count
		if len(rows) != len(testData) {
			t.Fatalf("Expected %d rows, got %d", len(testData), len(rows))
		}
		// Verify each row's data
		for i, row := range rows {
			expected := testData[i]

			if name, ok := row[0].(string); !ok || name != expected.name {
				t.Errorf("Row %d name mismatch: got %v, want %s", i, row[0], expected.name)
			}

			// json reads as float64
			if age, ok := row[1].(float64); !ok || int(age) != expected.age {
				t.Errorf("Row %d age mismatch: got %v, want %d", i, row[1], expected.age)
			}
		}
		// Commit
		if err := client.commit(); err != nil {
			t.Fatalf("Failed to commit read transaction: %v", err)
		}
	})
}

func TestDeltaClientReadInMemory(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	storage := NewFileObjectStorage(path.Join(tmpDir, "delta"))

	client1 := NewDeltaClient(&storage)

	// start new txn writer 1
	err := client1.nwTxn()
	if err != nil {
		t.Errorf("Failed to create transaction: %v", err)
		return
	}
	t.Logf("client1 txn: %+v", client1.txn)

	err = client1.createTable("table1", []string{"name", "age"})
	if err != nil {
		t.Errorf("Failed to create table: %v", err)
		return
	}
	t.Log("client1 create table")

	// write few rows which are not committed i.e. not flushed
	err = client1.writeRow("table1", []any{"Cal", 34})
	if err != nil {
		t.Errorf("Failed to write row: %v", err)
		return
	}
	t.Log("client1 write row1")

	err = client1.writeRow("table1", []any{"Harry", 40})
	if err != nil {
		t.Errorf("Failed to write row: %v", err)
		return
	}
	t.Log("client1 write row2")

	itr, err := client1.read("table1")
	if err != nil {
		t.Errorf("Failed to create the iterator:%v", err)
		return
	}

	ok, row := itr.next()
	for {
		t.Logf("first iteration ok: %v, row: %v", ok, row)
		if ok != false || row != nil {
			t.Logf("row: %v", row)
		} else {
			break
		}
		ok, row = itr.next()
	}

	err = client1.commit()
	if err != nil {
		t.Errorf("Failed to commit: %v", err)
		return
	}
	t.Log("client1 committed")

	// read again, this time from two different data files
	err = client1.nwTxn()
	if err != nil {
		t.Errorf("Failed to create second transaction: %v", err)
		return
	}
	// t.Logf("client1 second txn: %+v", client1.txn)

	itr, err = client1.read("table1")
	if err != nil {
		t.Errorf("Failed to create the second iterator:%v", err)
		return
	}

	ok, row = itr.next()
	for {
		t.Logf("second iteration ok: %v, row: %v", ok, row)
		if ok != false || row != nil {
			t.Logf("second iteration row: %v", row)
		} else {
			break
		}
		ok, row = itr.next()
	}

	// commit this should no anything as its read-only operation
	err = client1.commit()
	if err != nil {
		t.Errorf("Failed to commit read-only txn: %v", err)
		return
	}
	t.Log("client1 read-only txn committed")
}

func TestConcurrentDeltaClient(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	storage := NewFileObjectStorage(path.Join(tmpDir, "delta"))

	client1 := NewDeltaClient(&storage)
	client2 := NewDeltaClient(&storage)

	// start new txn writer 1
	err := client1.nwTxn()
	if err != nil {
		t.Errorf("Failed to create transaction: %v", err)
	}
	t.Logf("client1 txn: %+v", client1.txn)

	err = client1.createTable("table1", []string{"name", "age"})
	if err != nil {
		t.Errorf("Failed to create table: %v", err)
	}
	t.Log("client1 create table")

	err = client1.writeRow("table1", []any{"John", 30})
	if err != nil {
		t.Errorf("Failed to write row: %v", err)
	}
	t.Log("client1 write row")

	// start new txn writer 2
	err = client2.nwTxn()
	if err != nil {
		t.Errorf("Failed to create transaction: %v", err)
	}

	err = client2.createTable("table1", []string{"name", "age"})
	if err != nil {
		t.Errorf("Failed to create table: %v", err)
	}
	t.Log("client2 create table")

	err = client2.writeRow("table1", []any{"James", 30})
	if err != nil {
		t.Errorf("Failed to write row: %v", err)
		return
	}
	t.Log("client2 write row")

	// client2 commit
	err = client2.commit()
	if err != nil {
		t.Errorf("Client2 failed to commit: %v", err)
		return
	}
	t.Log("client2 committed")

	// should fail to commit
	err = client1.commit()
	if err != nil {
		t.Logf("Client1 expectedly failed to commit: %v", err)
		if !strings.Contains(err.Error(), "file exists") {
			t.Errorf("Expected file exists error, got: %v", err)
		}
		return
	}
	t.Log("client1 committed") // should not print
}

func TestDeltaClientRemoveRecord(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	storage := NewFileObjectStorage(path.Join(tmpDir, "delta"))

	client1 := NewDeltaClient(&storage)

	// start new txn writer 1
	err := client1.nwTxn()
	if err != nil {
		t.Errorf("Failed to create transaction: %v", err)
	}

	err = client1.createTable("table1", []string{"name", "age"})
	if err != nil {
		t.Errorf("Failed to create table: %v", err)
	}
	t.Log("client1 create table")

	err = client1.writeRow("table1", []any{"Tom", 22})
	if err != nil {
		t.Errorf("Failed to write row: %v", err)
	}
	err = client1.writeRow("table1", []any{"Clark", 30})
	if err != nil {
		t.Errorf("Failed to write row: %v", err)
	}
	// commit write
	client1.commit()

	//==============
	_ = client1.nwTxn()
	itr, err := client1.read("table1")
	if err != nil {
		t.Errorf("Failed to create the iterator:%v", err)
		return
	}

	ok, row := itr.next()
	for {
		if ok != false || row != nil {
			t.Logf("row: %v", row)
		} else {
			break
		}
		ok, row = itr.next()
	}

	client1.commit()
	//==============

	// new remove txn
	err = client1.nwTxn()
	if err != nil {
		t.Errorf("Failed to create transaction: %v", err)
	}

	client1.remove("table1", "age > 25")
	client1.commit()
	if err != nil {
		t.Errorf("Failed to commit: %v", err)
		return
	}
	t.Log("client1 remove closed")

	// new read txn
	err = client1.nwTxn()
	if err != nil {
		t.Errorf("Failed to create transaction: %v", err)
	}

	itr, err = client1.read("table1")
	if err != nil {
		t.Errorf("Failed to create the iterator:%v", err)
		return
	}

	ok, row = itr.next()
	for {
		if ok != false || row != nil {
			t.Logf("post remove row: %v", row)
		} else {
			break
		}
		ok, row = itr.next()
	}

	err = client1.commit()
	if err != nil {
		t.Errorf("Failed to commit: %v", err)
		return
	}
	t.Log("client1 read closed")
}
