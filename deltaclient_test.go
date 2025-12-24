package delta

import (
	"encoding/json"
	"os"
	"path"
	"slices"
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

func TestDeltaClient(t *testing.T) {
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

	err = client1.writeRow("table1", []any{"John", 30})
	if err != nil {
		t.Errorf("Failed to write row: %v", err)
		return
	}
	t.Log("client1 write row1")

	err = client1.writeRow("table1", []any{"Jack", 25})
	if err != nil {
		t.Errorf("Failed to write row: %v", err)
		return
	}
	t.Log("client1 write row2")

	// commit
	err = client1.commit()
	if err != nil {
		t.Errorf("Failed to commit: %v", err)
		return
	}
	t.Log("client1 committed")
}

func TestDeltaClientRead(t *testing.T) {
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

	err = client1.commit()
	if err != nil {
		t.Errorf("Failed to commit: %v", err)
		return
	}
	t.Log("client1 read-only txn committed")
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

func TestLogCheckpoint(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Setup storage and client
	storage := NewFileObjectStorage(path.Join(tmpDir, "delta"))
	client := NewDeltaClient(&storage)

	// Initialize transaction
	err := client.nwTxn()
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	//set the table & schema
	client.txn.table = "test"
	client.txn.schema = []string{"id", "name", "value"}

	// Create test data
	txnLogPath := path.Join(client.txn.table, logDir)
	testSchema := []string{"id", "name", "value"}

	// Create test transaction logs
	testCases := []struct {
		txnLogName string
		actions    []Action
	}{
		{
			txnLogName: "00000000000000000000.json",
			actions: []Action{
				{
					ChangeMetadataObject: &ChangeMetadataAction{
						Table:   "test",
						Columns: testSchema,
					},
				},
			},
		},
		{
			txnLogName: "00000000000000000001.json",
			actions: []Action{
				{
					DataActionObject: &DataAction{
						ActionType: "Add",
						Table:      "test",
						Name:       "file1.data",
						NumRows:    100,
					},
				},
			},
		},
		{
			txnLogName: "00000000000000000002.json",
			actions: []Action{
				{
					DataActionObject: &DataAction{
						ActionType: "Add",
						Table:      "test",
						Name:       "file2.data",
						NumRows:    150,
					},
				},
			},
		},
		{
			txnLogName: "00000000000000000003.json",
			actions: []Action{
				{
					DataActionObject: &DataAction{
						ActionType: "Remove",
						Table:      "test",
						Name:       "file1.data",
						NumRows:    100,
					},
				},
			},
		},
	}

	// Write test transaction logs
	for _, tc := range testCases {
		txn := transaction{
			table:   "test",
			schema:  testSchema,
			Actions: tc.actions,
		}

		txnBytes, err := json.Marshal(txn)
		if err != nil {
			t.Fatalf("Failed to marshal test transaction: %v", err)
		}

		txnPath := path.Join(txnLogPath, tc.txnLogName)
		err = storage.put(txnPath, txnBytes)
		if err != nil {
			t.Fatalf("Failed to write test transaction log: %v", err)
		}
	}

	// Test logCheckpoint with empty lastCheckpoint (process all logs)
	adds := make(map[Action]any)
	removes := make(map[Action]any)

	checkpoint, err := client.logCheckpoint(adds, removes, "")
	if err != nil {
		t.Fatalf("logCheckpoint failed: %v", err)
	}

	// Verify results
	if len(checkpoint.Add) != 1 {
		t.Errorf("Expected 1 add action, got %d", len(checkpoint.Add))
	} else if checkpoint.Add[0].DataActionObject.Name != "file2.data" {
		t.Errorf("Expected file2.data in add actions, got %s", checkpoint.Add[0].DataActionObject.Name)
	}

	if len(checkpoint.Remove) != 1 {
		t.Errorf(
			"Expected 1 remove action, got %d",
			len(checkpoint.Remove),
		)
	} else if checkpoint.Remove[0].DataActionObject.Name != "file1.data" {
		t.Errorf(
			"Expected file1.data in remove actions, got %s",
			checkpoint.Remove[0].DataActionObject.Name,
		)
	}

	if checkpoint.TotalActiveRows != 150 {
		t.Errorf("Expected 150 active rows, got %d", checkpoint.TotalActiveRows)
	}

	// Verify schema was properly set
	if len(checkpoint.Schema) != 3 || !slices.Equal(checkpoint.Schema, testSchema) {
		t.Errorf("Schema mismatch, expected %v, got %v", testSchema, checkpoint.Schema)
	}

	// Test with a non-empty lastCheckpoint (should only process logs after the checkpoint)
	adds = make(map[Action]any)
	removes = make(map[Action]any)

	// Set lastCheckpoint to the second log file
	checkpoint, err = client.logCheckpoint(adds, removes, "00000000000000000001.json")
	if err != nil {
		t.Fatalf("logCheckpoint with lastCheckpoint failed: %v", err)
	}

	// Should only process logs 2 and 3 (adding file2 and removing file1)
	if len(checkpoint.Add) != 1 || checkpoint.Add[0].DataActionObject.Name != "file2.data" {
		t.Errorf("Expected only file2.data in add actions after checkpoint")
	}
}

func TestGetOrCreateCheckpoint(t *testing.T) {
	t.Run("when checkpoint does not exist", func(t *testing.T) {
		tmpDir, cleanup := setupTestDir(t)
		defer cleanup()

		// Setup storage and client
		storage := NewFileObjectStorage(path.Join(tmpDir, "delta"))
		client := NewDeltaClient(&storage)

		// Initialize transaction
		err := client.nwTxn()
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		// Set up test data
		client.txn.table = "test"
		client.txn.schema = []string{"id", "name", "value"}

		// Create a test transaction log with an Add action
		txnLogPath := path.Join(client.txn.table, logDir, "00000000000000000000.json")
		txn := transaction{
			table:  "test",
			schema: client.txn.schema,
			Actions: []Action{{
				DataActionObject: &DataAction{
					ActionType: "Add",
					Table:      "test",
					Name:       "file1.data",
					NumRows:    100,
				},
			}},
		}

		txnBytes, err := json.Marshal(txn)
		if err != nil {
			t.Fatalf("Failed to marshal test transaction: %v", err)
		}

		err = storage.put(txnLogPath, txnBytes)
		if err != nil {
			t.Fatalf("Failed to write test transaction log: %v", err)
		}

		// Test getOrCreateCheckpoint when no checkpoint exists
		checkpoint, err := client.getOrCreateCheckpoint()
		if err != nil {
			t.Fatalf("getOrCreateCheckpoint failed: %v", err)
		}

		// Verify the checkpoint was created correctly
		if len(checkpoint.Add) != 1 {
			t.Errorf("Expected 1 add action, got %d", len(checkpoint.Add))
		} else if checkpoint.Add[0].DataActionObject.Name != "file1.data" {
			t.Errorf("Expected file1.data in add actions, got %s", checkpoint.Add[0].DataActionObject.Name)
		}

		if checkpoint.TotalActiveRows != 100 {
			t.Errorf("Expected 100 active rows, got %d", checkpoint.TotalActiveRows)
		}

		// Verify schema was properly set
		if len(checkpoint.Schema) != 3 || !slices.Equal(checkpoint.Schema, client.txn.schema) {
			t.Errorf("Schema mismatch, expected %v, got %v", client.txn.schema, checkpoint.Schema)
		}
	})

	t.Run("when checkpoint exists", func(t *testing.T) {
		tmpDir, cleanup := setupTestDir(t)
		defer cleanup()

		// Setup storage and client
		storage := NewFileObjectStorage(path.Join(tmpDir, "delta"))
		client := NewDeltaClient(&storage)

		// Initialize transaction
		err := client.nwTxn()
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		// Set up test data
		client.txn.table = "test"
		client.txn.schema = []string{"id", "name", "value"}

		// Create a checkpoint file
		checkpointData := Checkpoint{
			Table:  "test",
			Schema: client.txn.schema,
			Add: []Action{{
				DataActionObject: &DataAction{
					ActionType: "Add",
					Table:      "test",
					Name:       "existing_file.data",
					NumRows:    50,
				},
			}},
			TotalActiveRows: 50,
		}

		// Write the checkpoint file
		checkpointBytes, err := json.Marshal(checkpointData)
		if err != nil {
			t.Fatalf("Failed to marshal checkpoint data: %v", err)
		}

		// Create the _delta_log directory
		deltaLogPath := path.Join(client.txn.table, logDir)
		err = os.MkdirAll(path.Join(tmpDir, "delta", deltaLogPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create delta log directory: %v", err)
		}

		// Write the checkpoint file
		checkpointFileName := "00000000000000000000.checkpoint"
		checkpointPath := path.Join(deltaLogPath, checkpointFileName)
		err = storage.put(checkpointPath, checkpointBytes)
		if err != nil {
			t.Fatalf("Failed to write checkpoint file: %v", err)
		}

		// Write the _last_checkpoint file
		lastCheckpoint := LastCheckpoint{
			Checkpoint: checkpointFileName,
		}
		lastCheckpointBytes, err := json.Marshal(lastCheckpoint)
		if err != nil {
			t.Fatalf("Failed to marshal last checkpoint: %v", err)
		}

		lastCheckpointPath := path.Join(deltaLogPath, lastCheckPoint)
		err = storage.put(lastCheckpointPath, lastCheckpointBytes)
		if err != nil {
			t.Fatalf("Failed to write _last_checkpoint file: %v", err)
		}

		// Create a new transaction log that adds a file
		txnLogPath := path.Join(deltaLogPath, "00000000000000000001.json")
		txn := transaction{
			table:  "test",
			schema: client.txn.schema,
			Actions: []Action{{
				DataActionObject: &DataAction{
					ActionType: "Add",
					Table:      "test",
					Name:       "new_file.data",
					NumRows:    75,
				},
			}},
		}

		txnBytes, err := json.Marshal(txn)
		if err != nil {
			t.Fatalf("Failed to marshal test transaction: %v", err)
		}

		err = storage.put(txnLogPath, txnBytes)
		if err != nil {
			t.Fatalf("Failed to write test transaction log: %v", err)
		}

		// Test getOrCreateCheckpoint when a checkpoint exists
		checkpoint, err := client.getOrCreateCheckpoint()
		if err != nil {
			t.Fatalf("getOrCreateCheckpoint failed: %v", err)
		}

		// Verify the checkpoint was created correctly with combined data
		if len(checkpoint.Add) != 2 {
			t.Errorf("Expected 2 add actions, got %d", len(checkpoint.Add))
		} else {
			foundExisting := false
			foundNew := false
			for _, action := range checkpoint.Add {
				switch action.DataActionObject.Name {
				case "existing_file.data":
					foundExisting = true
				case "new_file.data":
					foundNew = true
				default:
					t.Logf("Unexpected file name: %s", action.DataActionObject.Name)
				}
			}
			if !foundExisting || !foundNew {
				t.Error("Expected both existing and new files in add actions")
			}
		}

		// Verify the total active rows is the sum of both files
		expectedRows := 125 // 50 (existing) + 75 (new)
		if checkpoint.TotalActiveRows != expectedRows {
			t.Errorf("Expected %d active rows, got %d", expectedRows, checkpoint.TotalActiveRows)
		}

		// Verify schema was properly set
		if len(checkpoint.Schema) != 3 || !slices.Equal(checkpoint.Schema, client.txn.schema) {
			t.Errorf("Schema mismatch, expected %v, got %v", client.txn.schema, checkpoint.Schema)
		}
	})
}
