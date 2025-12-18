package delta

type Checkpoint struct {
	Table           string
	ActiveLogs      []string // uuid
	TotalActiveRows int
}

func newCheckpoint(table string) *Checkpoint {
	return &Checkpoint{
		Table:           table,
		ActiveLogs:      make([]string, 0),
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

const checkpointFrequency = 10
