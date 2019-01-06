package params

type AsyncAction int

var (
	AACh = make(chan AsyncAction, 8)
)

const (
	AA_GET_LOCATION AsyncAction = iota
)
