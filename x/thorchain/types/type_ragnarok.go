package types

// IsEmpty whether the position is empty
func (m RagnarokWithdrawPosition) IsEmpty() bool {
	return m.Number < 0 || m.Pool.IsEmpty()
}
