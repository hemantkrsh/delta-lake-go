package delta

import (
	"encoding/json"
	"log"
	"path"
	"path/filepath"
	"slices"
)

type Checkpoint struct {
	Table           string
	Schema          []string
	Add             []Action // uuid
	Remove          []Action // uuid
	TotalActiveRows int
}

func newCheckpoint(table string) *Checkpoint {
	return &Checkpoint{
		Table:           table,
		Schema:          make([]string, 0),
		Add:             make([]Action, 0),
		Remove:          make([]Action, 0),
		TotalActiveRows: 0,
	}
}

type LastCheckpoint struct {
	Checkpoint string `json:"checkpoint"`
}

func newLastCheckpoint(name string) *LastCheckpoint {
	return &LastCheckpoint{
		Checkpoint: name,
	}
}

const checkpointFrequency = 2 //for testing else Defaults to 10

/*
iterate on the logs since last checkpoint(if exists using the _lastest_checkpoint file) else from the start.
leverage the iterate logic to remove the files with REMOVE and only keep the ADD
create the .checkpoint file using the putIfAbsent if present then its success
update the _latest_checkpoint file with the filename. put call
are there any concurrent writers issue - putifAbsent takes care of the checkpoint file,
if the index is taken then the putifAbsent will fail and the checkpoint belongs to the client which takes the index.
this makes sure that checkpoint is created by only one client. -- IMPLICIT
what about the update to _latest_checkpoint meta file -- this should be part of the checkpoint txn.
support for etag based put If-match -- Not required as the checkpoint is created by only one client.
*/
func (dc *deltaClient) triggerCheckpoint(index int) error {
	var checkpoint *Checkpoint
	var err error
	checkpoint, err = dc.getOrCreateCheckpoint()
	if err != nil {
		return err
	}

	log.Printf("activeRows: %d", checkpoint.TotalActiveRows)

	checkpointLog := getLogName(index, logExt, checkpointExt)
	checkpointBytes, err := dc.marshallCheckpoint(checkpoint.Schema, checkpoint.Add, checkpoint.Remove, checkpoint.TotalActiveRows)
	if err != nil {
		return err
	}

	//write checkpoint
	checkpointPath := path.Join(dc.txn.table, logDir, checkpointLog)
	err = dc.storage.putIfAbsent(checkpointPath, checkpointBytes)
	if err != nil {
		dc.txn = nil
		return err
	}

	//update _last_checkpoint file
	lastCheckpointLog := path.Join(dc.txn.table, logDir, lastCheckPoint)
	lastCheckpointData := &LastCheckpoint{
		Checkpoint: checkpointLog,
	}
	lastCheckpointDataBytes, err := json.Marshal(lastCheckpointData)
	if err != nil {
		return err
	}
	err = dc.storage.put(lastCheckpointLog, lastCheckpointDataBytes)
	if err != nil {
		return err
	}
	return nil
}

/*
get the previous checkpoint file name
read the checkpoint file
in case of no checkpoint return nil, nil, false
in case of error return nil, error, false
in case exists checkpointdata,nil, true
*/
func (dc *deltaClient) getOrCreateCheckpoint() (*Checkpoint, error) {
	checkPointPath := filepath.Join(dc.txn.table, logDir, lastCheckPoint)
	lastCheckpointExists, err := dc.storage.keyExists(checkPointPath)
	if err != nil {
		log.Printf("error in checking checkpoint exists: %v", err)
		return nil, err
	}

	var lastCheckpoint *LastCheckpoint
	addActions := make(map[Action]any)
	removeActions := make(map[Action]any)
	activeRows := 0

	if lastCheckpointExists {
		//read checkpoint
		lastCheckpointBytes, err := dc.storage.read(checkPointPath)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(lastCheckpointBytes, &lastCheckpoint)
		if err != nil {
			return nil, err
		}

		//last checkpoint
		previousCheckpoint, err := dc.readCheckpoint(lastCheckpoint.Checkpoint)
		if err != nil {
			return nil, err
		}
		activeRows = previousCheckpoint.TotalActiveRows

		//populate add & remove actions
		for _, action := range previousCheckpoint.Add {
			addActions[action] = 1
		}
		for _, action := range previousCheckpoint.Remove {
			removeActions[action] = 1
		}
	} else {
		lastCheckpoint = newLastCheckpoint("")
		log.Printf("no checkpoint found")
	}

	//latest checkpoint
	checkpoint, err := dc.logCheckpoint(addActions, removeActions, lastCheckpoint.Checkpoint)
	if err != nil {
		return nil, err
	}
	checkpoint.TotalActiveRows = checkpoint.TotalActiveRows + activeRows //add from previous checkpoint

	return checkpoint, nil
}

// Read all the files from the table and create the first checkpoint
// Handle the Remove actions and filter them out
func (dc *deltaClient) logCheckpoint(adds map[Action]any, removes map[Action]any, lastCheckpoint string) (*Checkpoint, error) {
	rowsSinceLastCheckpoint := 0
	table := dc.txn.table
	schema := dc.txn.schema
	addActions := make([]Action, 0)
	removeActions := make([]Action, 0)
	txnLogPath := path.Join(table, logDir)
	log.Printf("txnLogPath: %s", txnLogPath)
	txnLogs, err := dc.storage.listPrefix(txnLogPath, lastCheckpoint)
	if err != nil {
		return nil, err
	}

	slices.Sort(txnLogs)
	log.Printf("txnlogs count: %d", len(txnLogs))

	for _, txnLog := range txnLogs {
		bytes, err := dc.storage.read(path.Join(txnLogPath, txnLog))
		if err != nil {
			return nil, err
		}
		var oldTxn transaction
		err = json.Unmarshal(bytes, &oldTxn)
		if err != nil {
			return nil, err
		}

		// add prev actions
		// the remove could be handled here - the removed files may be filtered for dataobjects
		for _, action := range oldTxn.Actions {
			if action.DataActionObject != nil {
				//Add the appended logs
				switch action.DataActionObject.ActionType {
				case "Add":
					//addActions = append(addActions, action)
					adds[action] = 1
					rowsSinceLastCheckpoint += action.DataActionObject.NumRows
				case "Remove":
					removes[action] = 1
					//remove from add if exists
					for txn := range adds {
						if txn.DataActionObject.Name == action.DataActionObject.Name {
							delete(adds, txn)
							rowsSinceLastCheckpoint -= action.DataActionObject.NumRows
						}
					}
				}
			} else if action.ChangeMetadataObject != nil {
				// add schema -- overwrite and keep latest
				schema = action.ChangeMetadataObject.Columns
			} else {
				panic("Invalid action type")
			}
		}
	}

	for action := range adds {
		addActions = append(addActions, action)
	}

	for action := range removes {
		removeActions = append(removeActions, action)
	}

	log.Printf("Created checkpoint - Table: %s, Rows: %d, Schema: %v",
		dc.txn.table,
		rowsSinceLastCheckpoint,
		schema)

	//return checkpoint
	return &Checkpoint{
		Table:           dc.txn.table,
		Schema:          schema,
		Add:             addActions,
		Remove:          removeActions,
		TotalActiveRows: rowsSinceLastCheckpoint,
	}, nil
}

func (dc *deltaClient) marshallCheckpoint(schema []string, add []Action, remove []Action, activeRows int) ([]byte, error) {

	checkpoint := &Checkpoint{
		Table:           dc.txn.table,
		Schema:          schema,
		Add:             add,
		Remove:          remove,
		TotalActiveRows: activeRows,
	}

	bytes, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func (dc *deltaClient) readCheckpoint(checkpointLog string) (*Checkpoint, error) {
	var checkpoint Checkpoint
	path := path.Join(dc.txn.table, logDir, checkpointLog)
	checkPointBytes, err := dc.storage.read(path)
	if err != nil {
		log.Printf("error in reading checkpoint: %v", err)
		return nil, err
	}

	err = json.Unmarshal(checkPointBytes, &checkpoint)
	if err != nil {
		log.Printf("error in unmarshalling checkpoint: %v", err)
		return nil, err
	}
	return &checkpoint, nil

}
