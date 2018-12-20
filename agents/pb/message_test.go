package agents_pb

import "testing"

func TestType(t *testing.T) {
	m := NewMessage(AgentMessage_ADD_AS_TAB)
	t.Log(m)
	m = NewMessage(AgentMessage_GET_AS_TAB)
	t.Log(m)
	m = NewMessage(AgentMessage_COUNT_AS_TAB)
	t.Log(m)
}
