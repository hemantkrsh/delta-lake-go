package delta

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"
)

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

func TestTriggerCheckpointFlow(t *testing.T) {
	// Setup test directory
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Create storage and client
	storage := NewFileObjectStorage(path.Join(tempDir, "delta"))
	client := NewDeltaClient(&storage)

	// Create a new table
	tableName := "test_table"
	schema := []string{"id", "name", "value"}

	// Start transaction and create table
	err := client.nwTxn()
	if err != nil {
		t.Fatalf("Failed to start transaction: %v", err)
	}

	err = client.createTable(tableName, schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Write some test data - first batch
	testData1 := []any{1, "test1", 10.5}
	err = client.writeRow(tableName, testData1)
	if err != nil {
		t.Fatalf("Failed to write row: %v", err)
	}

	// Commit first transaction
	err = client.commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Start second transaction and write more data
	err = client.nwTxn()
	if err != nil {
		t.Fatalf("Failed to start second transaction: %v", err)
	}

	testData2 := []any{2, "test2", 20.5}
	err = client.writeRow(tableName, testData2)
	if err != nil {
		t.Fatalf("Failed to write second row: %v", err)
	}

	//commit
	err = client.commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify checkpoint was created
	checkpointPath := filepath.Join(tableName, logDir, "00000000000000000001.log.checkpoint")
	exists, err := storage.keyExists(checkpointPath)
	if err != nil {
		t.Fatalf("Error checking checkpoint existence: %v", err)
	}
	if !exists {
		t.Error("Checkpoint file was not created")
	}

	// Verify _last_checkpoint file
	lastCheckpointPath := filepath.Join(tableName, logDir, "_last_checkpoint")
	lastCheckpointData, err := storage.read(lastCheckpointPath)
	if err != nil {
		t.Fatalf("Failed to read _last_checkpoint: %v", err)
	}

	var lastCheckpoint LastCheckpoint
	err = json.Unmarshal(lastCheckpointData, &lastCheckpoint)
	if err != nil {
		t.Fatalf("Failed to unmarshal _last_checkpoint: %v", err)
	}

	expectedCheckpointName := "00000000000000000001.log.checkpoint"
	if lastCheckpoint.Checkpoint != expectedCheckpointName {
		t.Errorf("Unexpected checkpoint name in _last_checkpoint: %s, expected: %s",
			lastCheckpoint.Checkpoint, expectedCheckpointName)
	}

	// Before reading the checkpoint
	err = client.nwTxn()
	if err != nil {
		t.Fatalf("Failed to start transaction for reading checkpoint: %v", err)
	}

	// Set the table name for the transaction
	client.txn.table = tableName

	// Read the checkpoint
	checkpoint, err := client.readCheckpoint(lastCheckpoint.Checkpoint)
	if err != nil {
		t.Fatalf("Failed to read checkpoint: %v", err)
	}

	// Verify schema
	if !slices.Equal(checkpoint.Schema, schema) {
		t.Errorf("Schema mismatch, expected %v, got %v", schema, checkpoint.Schema)
	}

	// Verify number of active rows (should be 2)
	expectedRows := 2
	if checkpoint.TotalActiveRows != expectedRows {
		t.Errorf("Expected %d active rows, got %d", expectedRows, checkpoint.TotalActiveRows)
	}

	// Verify add actions
	if len(checkpoint.Add) != 2 {
		t.Errorf("Expected 2 add actions, got %d", len(checkpoint.Add))
	}

	// Verify no remove actions
	if len(checkpoint.Remove) != 0 {
		t.Errorf("Expected 0 remove actions, got %d", len(checkpoint.Remove))
	}
}
