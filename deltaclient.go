package delta

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path"
	"slices"

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
	ptr := dc.txn.writeBufferPointer
	if ptr == 0 {
		dc.txn.writeBuffer = &[DATA_OBJECT_SIZE][]any{}
		dc.txn.writeBufferPointer = 0 // todo: may be redundant
	}

	// flush if size is reached
	if dc.txn.writeBufferPointer >= DATA_OBJECT_SIZE {
		dc.flushRows(table)
		ptr = 0
	}

	dc.txn.writeBuffer[ptr] = row
	dc.txn.writeBufferPointer++
	// TODO: is the ptr pointing to correct num records

	return nil
}

func (dc *deltaClient) flushRows(table string) error {
	ptr := dc.txn.writeBufferPointer

	// if no data actions then return -- read-only txns
	if ptr == 0 {
		return nil
	}

	dataObject := dataObject{
		Data: dc.txn.writeBuffer[:ptr],
		Len:  dc.txn.writeBufferPointer,
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
		NumRows:    dc.txn.writeBufferPointer, // count of rows
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
	dc.txn.writeBufferPointer = 0

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
				if ok, err := row(currRow).filter(filterExpr.(simpleExpression)); err != nil {
					return err
				} else if ok {
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
	if dc.txn.writeBufferPointer == 0 {
		// there is no unflushed data
		return nil
	}
	filteredUnflushedRecords := &[DATA_OBJECT_SIZE][]any{}
	newUnflushedDataPointer := 0

	for ptr := range dc.txn.writeBufferPointer {
		currRow := dc.txn.writeBuffer[ptr]
		if ok, err := row(currRow).filter(filterExpr.(simpleExpression)); err != nil {
			return err
		} else if ok {
			filteredUnflushedRecords[newUnflushedDataPointer] = currRow
			newUnflushedDataPointer++
		}
	}

	// now switch the unflusheddata and ptr both
	dc.txn.writeBuffer = filteredUnflushedRecords
	dc.txn.writeBufferPointer = newUnflushedDataPointer
	return nil
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

	filteredActions := filterDeletes(dataActions)

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
		inMemoryRows:         dc.txn.writeBuffer,
		inMemoryRowsLen:      dc.txn.writeBufferPointer,
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
