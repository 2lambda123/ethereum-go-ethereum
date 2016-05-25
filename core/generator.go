package core

import "github.com/ethereum/go-ethereum/core/types"

// UnsealedBlock is a not-yet finalised block to which you can
// keep applying transactions until the gas-limit is met.
type UnsealedBlock struct {
	header       types.Header       // block header
	transactions types.Transactions // transactions previously applied
	uncles       []types.Header     // uncles previously applied

	gasPool *GasPool // gas pool for this block
}

// ApplyTransactions applies the given txs to the unsealed block and an error
// if unsuccessful.
func (b *UnsealedBlock) ApplyTransactions(txs types.Transactions) error {
	return nil
}

// ApplyUncles applies the given uncles to the unsealed block and an error
// if unsuccessful.
func (b *UnsealedBlock) ApplyUncles(uncles []types.Header) error {
	return nil
}

// SealedBlock is a finalised, immutable block.
type SealedBlock struct{}

// Sealer seals a block.
type Sealer interface{}

// Seal seals a block with the given Sealer and returns a new SealedBlock
// and/or an error.
func Seal(sealer Sealer, block UnsealedBlock) (*SealedBlock, error) {
	return nil, nil
}
