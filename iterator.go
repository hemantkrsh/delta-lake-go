package delta

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
