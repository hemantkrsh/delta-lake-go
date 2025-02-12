package delta

// Delta struct will contain the delta table functions - New, Append, Update, Delete

type Delta struct {
	tableName string
	path      string
}

func NewDeltaTable(tableName string, path string) *Delta {
	return &Delta{tableName, path}
	// todo: add transaction object;
	/**
	needs
	id
	name
	description
	schema
	partition columns
	metadata/table properties/configuration
	createdAt
	*/
}

func DeltaTable(tableName string) *Delta {
	return &Delta{tableName, ""}
	// todo: get the path and return the table with delta object
}

// write to have data and transaction object

type TableMetadata struct {
	id               string
	name             string
	description      string
	schema           Schema
	partitionColumns []string
	configuration    map[string]any
	createdAt        string
}
