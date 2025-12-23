package delta

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
	Checkpoint string
}

func newLastCheckpoint(name string) *LastCheckpoint {
	return &LastCheckpoint{
		Checkpoint: name,
	}
}

const checkpointFrequency = 2 //for testing else Defaults to 10
