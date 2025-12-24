package delta

import (
	"encoding/json"
	"path"
	"path/filepath"
	"slices"
	"testing"
)

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
