package fsm

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"github.com/hashicorp/raft"
)

// CommandPayload is payload sent by system when calling raft.Apply(cmd []byte, timeout time.Duration)
type CommandPayload struct {
	Operation string
	Key       string
	Value     interface{}
}

// ApplyResponse response from Apply raft
type ApplyResponse struct {
	Error error
	Data  interface{}
}

// snapshotNoop handle noop snapshot
type snapshotNoop struct{}

// Persist persist to disk. Return nil on success, otherwise return error.
func (s snapshotNoop) Persist(_ raft.SnapshotSink) error { return nil }

// Release release the lock after persist snapshot.
// Release is invoked when we are finished with the snapshot.
func (s snapshotNoop) Release() {}

// newSnapshotNoop is returned by an FSM in response to a snapshotNoop
// It must be safe to invoke FSMSnapshot methods with concurrent
// calls to Apply.
func newSnapshotNoop() (raft.FSMSnapshot, error) {
	return &snapshotNoop{}, nil
}

// badgerFSM raft.FSM implementation using badgerDB
type badgerFSM struct {
	db  *badger.DB
	log *slog.Logger
}

// get fetch data from badgerDB
func (b badgerFSM) get(key string) (interface{}, error) {
	var keyByte = []byte(key)
	var data interface{}

	txn := b.db.NewTransaction(false)
	defer func() {
		_ = txn.Commit()
	}()

	item, err := txn.Get(keyByte)
	if err != nil {
		data = map[string]interface{}{}
		return data, err
	}

	var value = make([]byte, 0)
	err = item.Value(func(val []byte) error {
		value = append(value, val...)
		return nil
	})

	if err != nil {
		data = map[string]interface{}{}
		return data, err
	}

	if len(value) > 0 {
		err = json.Unmarshal(value, &data)
	}

	if err != nil {
		data = map[string]interface{}{}
	}

	return data, err
}

// set store data to badgerDB
func (b badgerFSM) set(key string, value interface{}) error {
	var data = make([]byte, 0)
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if data == nil || len(data) <= 0 {
		return nil
	}

	txn := b.db.NewTransaction(true)
	err = txn.Set([]byte(key), data)
	if err != nil {
		txn.Discard()
		return err
	}

	return txn.Commit()
}

// delete remove data from badgerDB
func (b badgerFSM) delete(key string) error {
	var keyByte = []byte(key)

	txn := b.db.NewTransaction(true)
	err := txn.Delete(keyByte)
	if err != nil {
		return err
	}

	return txn.Commit()
}

// Apply log is invoked once a log entry is committed.
// It returns a value which will be made available in the
// ApplyFuture returned by Raft.Apply method if that
// method was called on the same Raft node as the FSM.
func (b badgerFSM) Apply(log *raft.Log) interface{} {
	switch log.Type {
	case raft.LogCommand:
		var payload = CommandPayload{}
		if err := json.Unmarshal(log.Data, &payload); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error marshalling store payload %s\n", err.Error())
			return nil
		}

		op := strings.ToUpper(strings.TrimSpace(payload.Operation))
		switch op {
		case "SET":
			return &ApplyResponse{
				Error: b.set(payload.Key, payload.Value),
				Data:  payload.Value,
			}
		case "GET":
			data, err := b.get(payload.Key)
			return &ApplyResponse{
				Error: err,
				Data:  data,
			}

		case "DELETE":
			return &ApplyResponse{
				Error: b.delete(payload.Key),
				Data:  nil,
			}
		}
	}

	_, _ = fmt.Fprintf(os.Stderr, "not raft log command type\n")
	return nil
}

// Snapshot will be called during make snapshot.
// Snapshot is used to support log compaction.
// No need to call snapshot since it already persisted in disk (using BadgerDB) when raft calling Apply function.
func (b badgerFSM) Snapshot() (raft.FSMSnapshot, error) {
	return newSnapshotNoop()
}

// Restore is used to restore an FSM from a Snapshot. It is not called
// concurrently with any other command. The FSM must discard all previous
// state.
// Restore will update all data in BadgerDB
func (b badgerFSM) Restore(rClose io.ReadCloser) error {
	defer func() {
		if err := rClose.Close(); err != nil {
			b.log.Error("error close restore", "error", err.Error())
		}
	}()

	b.log.Info("start restore")
	var totalRestored int

	decoder := json.NewDecoder(rClose)
	for decoder.More() {
		var data = &CommandPayload{}
		err := decoder.Decode(data)
		if err != nil {
			b.log.Error("error decode data", "error", err.Error())
			return err
		}

		if err := b.set(data.Key, data.Value); err != nil {
			b.log.Error("error persist data", "error", err.Error())
			return err
		}

		totalRestored++
	}

	// read closing bracket
	_, err := decoder.Token()
	if err != nil {
		b.log.Error("error read closing bracket", "error", err.Error())
		return err
	}

	b.log.Info("end restore", "total", totalRestored)
	return nil
}

// NewBadger raft.FSM implementation using badgerDB
func NewBadgerFSM(volumeDir string, logger *slog.Logger) (raft.FSM, error) {
	badgerOpt := badger.DefaultOptions(volumeDir)
	badgerDB, err := badger.Open(badgerOpt)
	if err != nil {
		return nil, err
	}

	return &badgerFSM{
		db:  badgerDB,
		log: logger,
	}, nil
}
