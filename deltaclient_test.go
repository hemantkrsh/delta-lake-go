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

func TestDeltaClientReadUncommitted(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	storage := NewFileObjectStorage(path.Join(tmpDir, "delta"))

	client1 := NewDeltaClient(&storage)

	testData := []struct {
		name string
		age  int
	}{
		{"Cal", 34},
		{"Harry", 40},
	}

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

	err = client1.commit()
	if err != nil {
		t.Fatalf("Failed to commit for table create: %v", err)
	}
	t.Log("table1 created")

	// start new txn for writing
	err = client1.nwTxn()
	if err != nil {
		t.Errorf("Failed to create transaction: %v", err)
		return
	}
	t.Logf("client1 txn: %+v", client1.txn)

	for _, data := range testData {
		// write data which are not committed i.e. not flushed
		err = client1.writeRow("table1", []any{data.name, data.age})
		if err != nil {
			t.Fatalf("Failed to write row: %v", err)
		}
		t.Log("client1 write row")
	}

	t.Run("Read uncommitted data", func(t *testing.T) {
		itr, err := client1.read("table1")
		if err != nil {
			t.Fatalf("Failed to create the iterator:%v", err)
		}

		var rows [][]any
		ok, row := itr.next()
		for {
			if ok != false || row != nil {
				rows = append(rows, row)
			} else {
				break
			}
			ok, row = itr.next()
		}
		//todo: add reading and comparing the data -- add test

		if len(rows) != len(testData) {
			t.Errorf("Expected %d rows, got %d", len(testData), len(rows))
		}

		for i, row := range rows {
			expected := testData[i]

			if name := row[0]; name != expected.name {
				t.Errorf("Row %d name mismatch: got %v, want %s", i, row[0], expected.name)
			}

			if age := row[1]; age != expected.age {
				t.Errorf("Row %d age mismatch: got %v, want %d", i, row[1], expected.age)
			}
		} // todo: a diff client will not be able to read uncommitted data -- add test for this

		//second client
		client2 := NewDeltaClient(&storage)

		// start new txn for reading
		err = client2.nwTxn()
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		itr2, err := client2.read("table1")

		if err != nil {
			t.Fatalf("Failed to create the iterator:%v", err)
		}

		ok2, row2 := itr2.next()
		if ok2 != false || row2 != nil {
			t.Fatalf("Second client should not be able to read uncommitted data")
		}

		err = client2.commit()
		if err != nil {
			t.Fatalf("Failed to commit read-only transaction: %v", err)
		}
	})

	//commit
	err = client1.commit()
	if err != nil {
		t.Fatalf("Failed to commit table1 write: %v", err)
	}
	t.Log("table1 write committed")

}

// func TestDeltaClientRepeatableRead(t *testing.T) {
// 	tmpDir, cleanup := setupTestDir(t)
// 	defer cleanup()
// 	storage := NewFileObjectStorage(path.Join(tmpDir, "delta"))

// 	client1 := NewDeltaClient(&storage)

// 	testData := []struct {
// 		name string
// 		age  int
// 	}{
// 		{"Cal", 34},
// 		{"Harry", 40},
// 	}

// 	// start a new txn
// 	err := client1.nwTxn()
// 	if err != nil {
// 		t.Errorf("Failed to create transaction: %v", err)
// 		return
// 	}
// 	t.Logf("client1 txn: %+v", client1.txn)

// 	err = client1.createTable("table1", []string{"name", "age"})
// 	if err != nil {
// 		t.Errorf("Failed to create table: %v", err)
// 		return
// 	}
// 	t.Log("client1 create table")

// 	err = client1.commit()
// 	if err != nil {
// 		t.Fatalf("Failed to commit for table create: %v", err)
// 	}
// 	t.Log("table1 created")

// 	// start a new txn for writing
// 	err = client1.nwTxn()
// 	if err != nil {
// 		t.Errorf("Failed to create transaction: %v", err)
// 		return
// 	}
// 	t.Logf("client1 txn: %+v", client1.txn)

// 	for _, data := range testData {
// 		// write data which are not committed i.e. not flushed
// 		err = client1.writeRow("table1", []any{data.name, data.age})
// 		if err != nil {
// 			t.Fatalf("Failed to write row: %v", err)
// 		}
// 		t.Log("client1 write row")
// 	}

// 	t.Run("Test repeatable read", func(t *testing.T) {
// 		itr, err := client1.read("table1")
// 		if err != nil {
// 			t.Fatalf("Failed to create client1's iterator:%v", err)
// 		}

// 		var rows [][]any
// 		ok, row := itr.next()
// 		for {
// 			if ok != false || row != nil {
// 				rows = append(rows, row)
// 			} else {
// 				break
// 			}
// 			ok, row = itr.next()
// 		}

// 		//same client gets read after write even for uncommitted data
// 		if len(rows) != len(testData) {
// 			t.Errorf("Expected %d rows, got %d", len(testData), len(rows))
// 		}

// 		for i, row := range rows {
// 			expected := testData[i]

// 			if name := row[0]; name != expected.name {
// 				t.Errorf("Row %d name mismatch: got %v, want %s", i, row[0], expected.name)
// 			}

// 			if age := row[1]; age != expected.age {
// 				t.Errorf("Row %d age mismatch: got %v, want %d", i, row[1], expected.age)
// 			}
// 		}

// 		//second client
// 		client2 := NewDeltaClient(&storage)

// 		// start new txn for reading
// 		err = client2.nwTxn()
// 		if err != nil {
// 			t.Fatalf("Failed to create client2's transaction: %v", err)
// 		}

// 		//commit
// 		err = client1.commit()
// 		if err != nil {
// 			t.Fatalf("Failed to commit client1's table1 write: %v", err)
// 		}
// 		t.Log("client1's table1 write committed")

// 		//read from second client will not return any rows even if client1 has committed
// 		// as this txn started after client1's commit
// 		itr2, err := client2.read("table1")

// 		if err != nil {
// 			t.Fatalf("Failed to create client2's iterator:%v", err)
// 		}

// 		ok2, row2 := itr2.next()
// 		if ok2 != false || row2 != nil {
// 			t.Fatalf("Client should not be able to read data from commit made after this client's transaction start")
// 		}

// 		err = client2.commit()
// 		if err != nil {
// 			t.Fatalf("Failed to commit read-only transaction: %v", err)
// 		}
// 	})

// }

func TestDeltaClientConcurrentWrites(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	storage := NewFileObjectStorage(path.Join(tmpDir, "delta"))

	client1 := NewDeltaClient(&storage)
	client2 := NewDeltaClient(&storage)

	t.Run("Concurrent clients with conflicting writes", func(t *testing.T) {
		// start a new txn for writer 1
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

		// start a new txn for writer 2
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
		t.Errorf("client1 committed unexpectedly - should have failed with file exists error")
	})
}

func TestDeltaClientRemoveRecord(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	storage := NewFileObjectStorage(path.Join(tmpDir, "delta"))

	client1 := NewDeltaClient(&storage)

	// Setup table with test data
	{
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
	}

	t.Run("Verify initial data", func(t *testing.T) {
		_ = client1.nwTxn()
		itr, err := client1.read("table1")
		if err != nil {
			t.Errorf("Failed to create the iterator:%v", err)
			return
		}

		var rows [][]any
		ok, row := itr.next()
		for {
			if ok != false || row != nil {
				rows = append(rows, row)
			} else {
				break
			}
			ok, row = itr.next()
		}

		client1.commit()

		// Verify we have 2 rows initially
		if len(rows) != 2 {
			t.Errorf("Expected 2 rows initially, got %d", len(rows))
		}

		// Verify the data
		expectedData := [][]any{{"Tom", 22.0}, {"Clark", 30.0}}
		for i, row := range rows {
			if len(row) != 2 {
				t.Errorf("Row %d should have 2 columns, got %d", i, len(row))
				continue
			}
			if row[0] != expectedData[i][0] {
				t.Errorf("Row %d name mismatch: got %v, want %v", i, row[0], expectedData[i][0])
			}
			if row[1] != expectedData[i][1] {
				t.Errorf("Row %d age mismatch: got %v, want %v", i, row[1], expectedData[i][1])
			}
		}
	})

	t.Run("Remove records and verify", func(t *testing.T) {
		// new remove txn
		err := client1.nwTxn()
		if err != nil {
			t.Errorf("Failed to create transaction: %v", err)
		}

		client1.remove("table1", "age > 25")
		client1.commit()
		if err != nil {
			t.Errorf("Failed to commit: %v", err)
			return
		}
		t.Log("client1 remove committed")

		// new read txn
		err = client1.nwTxn()
		if err != nil {
			t.Errorf("Failed to create transaction: %v", err)
		}

		itr, err := client1.read("table1")
		if err != nil {
			t.Errorf("Failed to create the iterator:%v", err)
			return
		}

		var rows [][]any
		ok, row := itr.next()
		for {
			if ok != false || row != nil {
				rows = append(rows, row)
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
		t.Log("client1 read committed")

		// Verify we have 1 row after removal (Tom with age 22)
		if len(rows) != 1 {
			t.Errorf("Expected 1 row after removal, got %d", len(rows))
		}

		// Verify the remaining data is Tom (age 22)
		if len(rows) > 0 {
			expectedRow := []any{"Tom", 22.0}
			actualRow := rows[0]
			if len(actualRow) != 2 {
				t.Errorf("Remaining row should have 2 columns, got %d", len(actualRow))
			} else {
				if actualRow[0] != expectedRow[0] {
					t.Errorf("Remaining row name mismatch: got %v, want %v", actualRow[0], expectedRow[0])
				}
				if actualRow[1] != expectedRow[1] {
					t.Errorf("Remaining row age mismatch: got %v, want %v", actualRow[1], expectedRow[1])
				}
			}
		}
	})
}
