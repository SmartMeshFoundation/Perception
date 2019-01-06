package params

type AsyncAction int

type AA struct {
	Action  AsyncAction
	Payload interface{}
}

func NewAA(a AsyncAction, p interface{}) *AA {
	return &AA{a, p}
}

var (
	AACh = make(chan *AA, 8)
)

const (
	AA_GET_MY_LOCATION AsyncAction = iota
	AA_GET_AS_LOCATION AsyncAction = iota
)
