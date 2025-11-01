package delta

import (
	"os"
	"path"
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
	t.Logf("client1 second txn: %+v", client1.txn)

	itr, err = client1.read("table1")
	if err != nil {
		t.Errorf("Failed to create the second iterator:%v", err)
		return
	}

	ok, row = itr.next()
	for {
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
		t.Errorf("Failed to commit: %v", err)
		return
	}
	t.Log("client2 committed")

	// should fail to commit
	err = client1.commit()
	if err != nil {
		t.Errorf("Failed to commit: %v", err)
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
