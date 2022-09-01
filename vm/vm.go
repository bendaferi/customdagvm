package vm

import (
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/database/versiondb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowstorm"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/version"
	"github.com/gorilla/rpc/v2"
)

type VM struct {
	ctx *snow.Context
	bootstrapped bool
	feeAssetID ids.ID
	txs          []snowstorm.Tx
	toEngine     chan<- common.Message
	baseDB database.Database
	db     *versiondb.Database
}

func (vm *VM) Initialize(
	ctx *snow.Context,
	dbManager manager.Manager,
	genesisBytes []byte,
	upgradeBytes []byte,
	_ []byte, // configBytes
	toEngine chan<- common.Message,
	_ []*common.Fx, // fxs
	_ common.AppSender,
) error {
	db := dbManager.Current().Database
	vm.ctx = ctx
	vm.toEngine = toEngine
	vm.baseDB = db
	vm.db = versiondb.New(db)

	return vm.db.Commit()
}

func (vm *VM) onBootstrapStarted() error {
	return nil
}

func (vm *VM) onNormalOperationsStarted() error {
	vm.bootstrapped = true
	return nil
}

func (vm *VM) SetState(state snow.State) error {
	switch state {
	case snow.Bootstrapping:
		return vm.onBootstrapStarted()
	case snow.NormalOp:
		return vm.onNormalOperationsStarted()
	default:
		return snow.ErrUnknownState
	}
}

func (vm *VM) Shutdown() error {
	return vm.baseDB.Close()
}

func (vm *VM) Version() (string, error) {
	return version.Current.String(), nil
}

func (vm *VM) CreateHandlers() (map[string]*common.HTTPHandler, error) {
	codec := json.NewCodec()

	rpcServer := rpc.NewServer()
	rpcServer.RegisterCodec(codec, "application/json")
	rpcServer.RegisterCodec(codec, "application/json;charset=UTF-8")
	if err := rpcServer.RegisterService(&Service{vm: vm}, "customdagvm"); err != nil {
		return nil, err
	}

	return map[string]*common.HTTPHandler{
		"":        {Handler: rpcServer},
	}, nil
}

func (vm *VM) CreateStaticHandlers() (map[string]*common.HTTPHandler, error) {
	newServer := rpc.NewServer()
	codec := json.NewCodec()
	newServer.RegisterCodec(codec, "application/json")
	newServer.RegisterCodec(codec, "application/json;charset=UTF-8")

	staticService := CreateStaticService()
	return map[string]*common.HTTPHandler{
		"": {LockOptions: common.WriteLock, Handler: newServer},
	}, newServer.RegisterService(staticService, "customdagvm")
}

func (vm *VM) PendingTxs() []snowstorm.Tx {
	txs := vm.txs
	vm.txs = nil
	return txs
}

func (vm *VM) ParseTx(b []byte) (snowstorm.Tx, error) {
	return vm.parseTx(b)
}

func (vm *VM) GetTx(txID ids.ID) (snowstorm.Tx, error) {
	tx := &UniqueTx{
		vm:   vm,
		txID: txID,
	}
	// Verify must be called in the case the that tx was flushed from the unique
	// cache.
	return tx, tx.verifyWithoutCacheWrites()
}

func (vm *VM) parseTx(bytes []byte) (*UniqueTx, error) {
	rawTx, err := vm.parser.Parse(bytes)
	if err != nil {
		return nil, err
	}

	tx := &UniqueTx{
		TxCachedState: &TxCachedState{
			Tx: rawTx,
		},
		vm:   vm,
		txID: rawTx.ID(),
	}
	if err := tx.SyntacticVerify(); err != nil {
		return nil, err
	}

	if tx.Status() == choices.Unknown {
		if err := vm.state.PutTx(tx.ID(), tx.Tx); err != nil {
			return nil, err
		}
		if err := tx.setStatus(choices.Processing); err != nil {
			return nil, err
		}
		return tx, vm.db.Commit()
	}

	return tx, nil
}