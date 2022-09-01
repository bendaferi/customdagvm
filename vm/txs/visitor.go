// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import "github.com/ava-labs/avalanchego/vms/components/avax"

var _ Visitor = &utxoGetter{}

// Allow vm to execute custom logic against the underlying transaction types.
type Visitor interface {
	BaseTx(*BaseTx) error
}

// utxoGetter returns the UTXOs transaction is producing.
type utxoGetter struct {
	tx    *Tx
	utxos []*avax.UTXO
}

func (u *utxoGetter) BaseTx(tx *BaseTx) error {
	txID := u.tx.ID()
	u.utxos = make([]*avax.UTXO, len(tx.Outs))
	for i, out := range tx.Outs {
		u.utxos[i] = &avax.UTXO{
			UTXOID: avax.UTXOID{
				TxID:        txID,
				OutputIndex: uint32(i),
			},
			Asset: avax.Asset{ID: out.AssetID()},
			Out:   out.Out,
		}
	}
	return nil
}
