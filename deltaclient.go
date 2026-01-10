package delta

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/uuid"
)

type deltaClient struct {
	storage objectStorage
	txn     *transaction
}

type dataObject struct {
	Data [][]any
	Len  int
}

var (
	errTxnExists           = errors.New("transaction already exists")
	errNoTxn               = errors.New("no transaction exists")
	errTableExists         = errors.New("table already exists")
	errTableNotFound       = errors.New("table not found")
	errSchemaCannotBeEmpty = errors.New("schema cannot be empty")
	errSchemaMismatch      = errors.New("schema mismatch")
	errIncorrectExprFormat = errors.New("query is not formatted correctly")
)

func NewDeltaClient(storage objectStorage) deltaClient {
	return deltaClient{
		storage: storage,
		txn:     nil,
	}
}

func (dc *deltaClient) nwTxn() error {
	if dc.txn != nil {
		return errTxnExists
	}
	dc.txn = NewTransaction() // new txn
	return nil
}

func (dc *deltaClient) nwTableTxn(table string) error {
	// set the txn table
	dc.txn.table = table
	txnLogPath := path.Join(table, logDir)
	log.Printf("txnLogPath: %s", txnLogPath)
	txnLogs, err := dc.storage.listPrefix(txnLogPath, "")
	if err != nil {
		return err
	}

	slices.Sort(txnLogs)
	log.Printf("txnlogs count: %d", len(txnLogs))
	for _, txnLog := range txnLogs {
		bytes, err := dc.storage.read(path.Join(txnLogPath, txnLog))
		if err != nil {
			return err
		}
		var oldTxn transaction
		err = json.Unmarshal(bytes, &oldTxn)
		if err != nil {
			return err
		}
		log.Printf("old txn:%+v", oldTxn)
		// set new txn id
		dc.txn.TxnId = oldTxn.TxnId + 1
	}

	//use checkpointing mechanism to populate prev actions and schema
	checkpoint, err := dc.getOrCreateCheckpoint()
	if err != nil {
		log.Printf("error in getOrCreateCheckpoint: %v", err)
		return err
	}

	log.Printf("checkpoint: %+v", checkpoint)

	dc.txn.prevActions = append(dc.txn.prevActions, checkpoint.Add...)
	dc.txn.schema = append(dc.txn.schema, checkpoint.Schema...)

	return nil
}

func (dc *deltaClient) createTable(table string, schema []string) error {
	if dc.txn == nil {
		return errNoTxn
	}

	if len(schema) == 0 {
		return errSchemaCannotBeEmpty
	}

	dc.nwTableTxn(table)

	log.Printf("txn id:%d", dc.txn.TxnId)
	log.Printf("schema:%v", dc.txn.schema)

	if len(dc.txn.schema) > 0 {
		return errTableExists
	}
	dc.txn.Actions = append(dc.txn.Actions, Action{
		ChangeMetadataObject: &ChangeMetadataAction{
			Table:   table,
			Columns: schema,
		},
	})
	dc.txn.schema = schema // for quick access to schema for recently written data
	fmt.Printf("create table txn: %+v", dc.txn)
	return nil
}

func (dc *deltaClient) writeRow(table string, row []any) error {
	if dc.txn == nil {
		return errNoTxn
	}

	// no schema ~ txn is not init with latest txn info for table
	// not to call nwTxnTable if first read and then writeRow as tbl txn is already init
	if len(dc.txn.schema) == 0 {
		dc.nwTableTxn(table)
	}

	if len(dc.txn.schema) == 0 {
		return errTableNotFound
	}

	// check if schema matches
	if len(dc.txn.schema) != len(row) {
		return fmt.Errorf("%w: expected %d columns, got %d", errSchemaMismatch, len(dc.txn.schema), len(row))
	}

	// Initialize unflushed data if not already initialized
	ptr := dc.txn.unflushedDataPointer
	if ptr == 0 {
		dc.txn.unflushedData = &[DATA_OBJECT_SIZE][]any{}
		dc.txn.unflushedDataPointer = 0 // todo: may be redundant
	}

	// flush if size is reached
	if dc.txn.unflushedDataPointer >= DATA_OBJECT_SIZE {
		dc.flushRows(table)
		ptr = 0
	}

	dc.txn.unflushedData[ptr] = row
	dc.txn.unflushedDataPointer++
	// TODO: is the ptr pointing to correct num records

	return nil
}

func (dc *deltaClient) flushRows(table string) error {
	ptr := dc.txn.unflushedDataPointer

	// if no data actions then return -- read-only txns
	if ptr == 0 {
		return nil
	}

	dataObject := dataObject{
		Data: dc.txn.unflushedData[:ptr],
		Len:  dc.txn.unflushedDataPointer,
	}

	bytes, err := json.Marshal(dataObject)
	if err != nil {
		return err
	}

	name := fmt.Sprintf("%s%s", uuid.New().String(), dataExt)

	// create the data object for data action
	dataAction := &DataAction{
		ActionType: "Add",
		Table:      table,
		Name:       name,
		NumRows:    dc.txn.unflushedDataPointer, // count of rows
	}

	// write the data object
	// dataFile := fmt.Sprintf("%s%s", dataAction.Name, dataExt)
	err = dc.storage.putObject(path.Join(table, name), bytes)
	if err != nil {
		return err
	}

	// add the data action to the txn
	dc.txn.Actions = append(dc.txn.Actions, Action{
		DataActionObject: dataAction,
	})
	fmt.Printf("flushRows txn: %+v", dc.txn)

	// reset the unflushed data
	dc.txn.unflushedDataPointer = 0

	return nil
}

func (dc *deltaClient) remove(table string, predicate string) error {
	if dc.txn == nil {
		return errNoTxn
	}

	// no schema ~ txn is not init with latest txn info for table
	// not to call nwTxnTable if first read and then writeRow as tbl txn is already init
	if len(dc.txn.schema) == 0 {
		dc.nwTableTxn(table)
	}

	if len(dc.txn.schema) == 0 {
		return errTableNotFound
	}

	// get schema and create the simple expression
	schema := dc.txn.schema
	filterExpr, err := getSimpleExpr(schema, predicate)
	if err != nil {
		return err
	}
	// use filterExpr
	log.Printf("remove filter expr: %+v", filterExpr)

	dc.filterInMemoryRecords(filterExpr)
	dc.filterDataObjects(filterExpr)
	// first in-memory recods are filtered then other files on by one
	// action as well as previous action (if the data is not flished else only previous will do)
	return nil
}

func (dc *deltaClient) filterDataObjects(filterExpr expression) error {
	// previous actions have the dataobjects
	var rowsRemoved bool
	var rowsRemovedCount int
	var fileName string

	for idx := range dc.txn.prevActions {
		// prevActions only contain dataobjectactions
		rowsRemoved = false
		action := dc.txn.prevActions[idx]
		log.Printf("prev action %d: %+v \n", idx, action)
		if action.DataActionObject.ActionType == "Add" {
			// apply filter
			fileName = action.DataActionObject.Name
			dataObject, err := dc.readDataObject(fileName)
			log.Printf("data object:%+v", dataObject.Data)
			if err != nil {
				return err
			}
			for objIdx := range dataObject.Len {
				currRow := dataObject.Data[objIdx]
				log.Printf("curr row %d:%+v", objIdx, currRow)
				if row(currRow).filter(filterExpr.(simpleExpression)) {
					log.Printf("row filtered:%+v", currRow...)
					if !rowsRemoved {
						rowsRemoved = true
					}
					rowsRemovedCount++
				} else {
					log.Printf("row not filtered filtered:%+v", currRow...)
					dc.writeRow(dc.txn.table, currRow)
					// TODO: use write??
				}

			}

		}
		// if rowsRemoved create RemoveDataObject Action else no change
		if rowsRemoved {
			dataAction := &DataAction{
				ActionType: "Remove",
				Table:      dc.txn.table,
				Name:       fileName,
				NumRows:    rowsRemovedCount, // count of rows - get the count
			}
			log.Printf("records removed: %+v", dataAction)
			dc.txn.Actions = append(dc.txn.Actions, Action{DataActionObject: dataAction})
		}

	}
	return nil
}

func (dc *deltaClient) filterInMemoryRecords(filterExpr expression) error {
	if dc.txn.unflushedDataPointer == 0 {
		// there is no unflushed data
		return nil
	}
	filteredUnflushedRecords := &[DATA_OBJECT_SIZE][]any{}
	newUnflushedDataPointer := 0

	for ptr := range dc.txn.unflushedDataPointer {
		currRow := dc.txn.unflushedData[ptr]
		if row(currRow).filter(filterExpr.(simpleExpression)) {
			filteredUnflushedRecords[newUnflushedDataPointer] = currRow
			newUnflushedDataPointer++
		}
	}

	// now switch the unflusheddata and ptr both
	dc.txn.unflushedData = filteredUnflushedRecords
	dc.txn.unflushedDataPointer = newUnflushedDataPointer
	return nil
}

func getSimpleExpr(schema []string, predicate string) (simpleExpression, error) {
	// simple expression is of type col > | < | == | != value
	predicateTokens := strings.Split(predicate, " ")
	if len(predicateTokens) < 3 {
		return simpleExpression{}, errIncorrectExprFormat
	}
	colIdx, err := getColIndex(schema, predicateTokens[0])
	if err != nil {
		return simpleExpression{}, err
	}
	expr := simpleExpression{
		left:     colIdx,
		operator: Operator(strings.TrimSpace(predicateTokens[1])),
		right:    expression(strings.TrimSpace(predicateTokens[2])),
	}
	return expr, nil
}

func getColIndex(schema []string, col string) (int, error) {
	for idx := range schema {
		if strings.Compare(schema[idx], col) == 0 {
			return idx, nil
		}
	}
	return 0, nil
}

// One txn for write to an existing table
func (dc *deltaClient) commit() error {
	if dc.txn == nil {
		return errNoTxn
	}

	log.Printf("commit txn:%+v", dc.txn)

	// flush any pending rows
	table := dc.txn.table
	err := dc.flushRows(table)
	if err != nil {
		dc.txn = nil
		return err
	}

	log.Printf("commit txn actions:%+v", dc.txn.Actions)

	// if read-only txn then return immediately
	if len(dc.txn.Actions) == 0 {
		dc.txn = nil
		return nil
	}

	var data []byte
	for range dc.txn.Actions {
		dc.txn.prevActions = nil
		data, err = json.Marshal(dc.txn)
		if err != nil {
			dc.txn = nil
			return err
		}
	}
	// write the txn
	txnLog := fmt.Sprintf("%020d%s", dc.txn.TxnId, logExt)
	log.Printf("txnLog: %s", txnLog)
	err = dc.storage.putIfAbsent(path.Join(table, logDir, txnLog), data)
	if err != nil {
		dc.txn = nil
		return err
	}

	//create checkpoint every 10th txn once the txn is committed, this way the client that commits the txn, also commits the checkpoint
	//with same index i.e. until that txn log the actions are compacted
	if dc.txn.TxnId != 0 && (dc.txn.TxnId+1)%checkpointFrequency == 0 {
		err := dc.triggerCheckpoint(dc.txn.TxnId)
		if err != nil {
			dc.txn = nil
			log.Printf("failed to create checkpoint: %v", err) //only log
		}
	}

	dc.txn = nil //this should be cleared anyways
	return nil
}

type rowIterator struct {
	deltaClient     *deltaClient
	inMemoryRows    *[DATA_OBJECT_SIZE][]any
	inMemoryRowsLen int
	inMemoryRowsPtr int

	dataObjects    []dataObject
	dataObjectsPtr int

	currentDataObject    *dataObject
	currentDataObjectLen int
	currentDataObjectPtr int
}

func (itr *rowIterator) next() (bool, []any) {
	// start with in-memory rows
	if itr.inMemoryRowsPtr < itr.inMemoryRowsLen {
		row := itr.inMemoryRows[itr.inMemoryRowsPtr]
		itr.inMemoryRowsPtr++
		return true && len(itr.dataObjects) > 0, row
		// true if ptr<len || dataObjects exists that means there is at least 1 more row
	}

	// action -> dataObject ==> rows
	if itr.dataObjectsPtr < len(itr.dataObjects) {
		itr.currentDataObject = &itr.dataObjects[itr.dataObjectsPtr]
	}

	if itr.dataObjectsPtr == len(itr.dataObjects) {
		itr = nil
		return false, nil
	}

	if itr.currentDataObject != nil {
		if itr.currentDataObjectPtr == itr.currentDataObject.Len {
			itr.dataObjectsPtr++
			itr.currentDataObjectPtr = 0 // reset the ptr to curr data object
			return itr.next()
		}
		if itr.currentDataObjectPtr < itr.currentDataObject.Len {
			row := itr.currentDataObject.Data[itr.currentDataObjectPtr]
			itr.currentDataObjectPtr++
			moreRows := itr.currentDataObjectPtr < itr.currentDataObject.Len ||
				len(itr.dataObjects)-1 > itr.dataObjectsPtr
				// needs re-work
			return moreRows, row
			// more dataObjects or more rows
		}
	}

	return false, nil
}

func (dc *deltaClient) read(table string) (*rowIterator, error) {
	if dc.txn == nil {
		return nil, errNoTxn
	}

	// if no table txn has been done until now then init otherwise no
	if len(dc.txn.schema) == 0 {
		if err := dc.nwTableTxn(table); err != nil {
			log.Printf("failed to create table txn: %v", err)
			return nil, err
		}
	}

	log.Printf("read txn:%+v", dc.txn)
	// read from the unflushed data first
	// read from the data files
	dataActions := make([]Action, 0, 5)

	dataObjectsToRead := make([]dataObject, 0, 5)

	//TODO: the prevAction can get active logs starting at the checkpoint
	for _, action := range dc.txn.prevActions {
		if (action.DataActionObject != nil && action.DataActionObject.ActionType == "Add") ||
			(action.DataActionObject != nil && action.DataActionObject.ActionType == "Remove") {
			dataActions = append(dataActions, action)
		}
	}

	filteredActions := dc.filterDeletes(dataActions)

	for _, action := range filteredActions {
		daObject, err := dc.readDataObject(action.DataActionObject.Name)
		if err != nil {
			return nil, err
		}
		dataObjectsToRead = append(dataObjectsToRead, *daObject)
	}

	// filter if the same file is part of Remove and Add
	itr := &rowIterator{
		deltaClient:          dc,
		inMemoryRows:         dc.txn.unflushedData,
		inMemoryRowsLen:      dc.txn.unflushedDataPointer,
		inMemoryRowsPtr:      0,
		dataObjects:          dataObjectsToRead,
		dataObjectsPtr:       0,
		currentDataObjectLen: 0,
		currentDataObjectPtr: 0,
		currentDataObject:    nil,
	}

	return itr, nil
}

func (dc *deltaClient) readDataObject(name string) (*dataObject, error) {
	dObjectKey := path.Join(dc.txn.table, name)
	bytes, err := dc.storage.read(dObjectKey)
	if err != nil {
		return nil, err
	}
	var dObject *dataObject
	err = json.Unmarshal(bytes, &dObject)
	if err != nil {
		return nil, err
	}
	return dObject, nil
}

func (dc *deltaClient) filterDeletes(actions []Action) []Action {
	removedFileMap := make(map[string]struct{})
	var filteredActions []Action

	for _, action := range actions {
		if action.DataActionObject.ActionType == "Remove" {
			removedFileMap[action.DataActionObject.Name] = struct{}{}
		}
	}
	// loop and delete the actions of type Add and Remove for a table file
	for _, action := range actions {
		if _, exists := removedFileMap[action.DataActionObject.Name]; !exists {
			filteredActions = append(filteredActions, action)
		}
	}
	return filteredActions
}

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

	checkpointLog := dc.getLogName(index, logExt, checkpointExt)
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

func (dc *deltaClient) getLogName(index int, exts ...string) string {
	return fmt.Sprintf("%020d%s", index, strings.Join(exts, ""))
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
