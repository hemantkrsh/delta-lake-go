package delta

const DATA_OBJECT_SIZE = 1000 * 64

type DataAction struct {
	ActionType string // Add or Remove
	Table      string
	Name       string // uuid
	NumRows    int
}

type ChangeMetadataAction struct {
	Table   string
	Columns []string
}

// enum
type Action struct {
	DataActionObject     *DataAction
	ChangeMetadataObject *ChangeMetadataAction
}

type transaction struct {
	TxnId       int
	table       string
	Actions     []Action // table -> actions
	prevActions []Action // table -> previous actions

	schema []string // table -> column mapping

	// table -> array of slices of any
	// store the data actions until data_object_size and then flush
	writeBuffer        *[DATA_OBJECT_SIZE][]any
	writeBufferPointer int
}

func NewTransaction() *transaction {
	return &transaction{
		TxnId:              0,
		Actions:            make([]Action, 0, 1),
		prevActions:        make([]Action, 0, 1),
		schema:             make([]string, 0, 1),
		writeBufferPointer: 0,
	}
}
