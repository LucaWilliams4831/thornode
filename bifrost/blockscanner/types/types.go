package types

import "fmt"

var (
	ErrUnavailableBlock        = fmt.Errorf("block is not yet available")
	ErrFailOutputMatchCriteria = fmt.Errorf("fail to get output matching criteria")
)
