package types

import (
	"fmt"
	"strconv"
	"strings"
)

func (m *Node) String() string {
	return fmt.Sprintf("Node: %s", m.Pubkey)
}

// IsEmpty check whether it is empty
func (m *Blame) IsEmpty() bool {
	return len(m.FailReason) == 0
}

// String implement fmt.Stringer
func (m *Blame) String() string {
	sb := strings.Builder{}
	sb.WriteString("reason:" + m.FailReason + " is_unicast:" + strconv.FormatBool(m.IsUnicast) + "\n")
	sb.WriteString(fmt.Sprintf("nodes:%+v\n", m.BlameNodes))
	sb.WriteString(fmt.Sprintf("is unicast:%+v\n", m.IsUnicast))
	return sb.String()
}
