package delta

func assert(b bool, msg string) {
	if !b {
		panic(msg)
	}
}

func assertEqual[T comparable](a, b T, msg string) {
	if a != b {
		panic(msg)
	}
}
