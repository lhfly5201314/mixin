package storage

import (
	"encoding/binary"

	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

func (s *BadgerStore) ReadSnapshotsSinceTopology(topologyOffset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	snapshots := make([]*common.SnapshotWithTopologicalOrder, 0)
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	it.Seek(graphTopologyKey(topologyOffset))
	for ; it.ValidForPrefix([]byte(graphPrefixTopology)) && uint64(len(snapshots)) < count; it.Next() {
		item := it.Item()
		v, err := item.ValueCopy(nil)
		if err != nil {
			return snapshots, err
		}
		topology := graphTopologyOrder(item.Key())
		item, err = txn.Get(v)
		if err != nil {
			return snapshots, err
		}
		v, err = item.ValueCopy(nil)
		if err != nil {
			return snapshots, err
		}
		var snap common.SnapshotWithTopologicalOrder
		err = msgpack.Unmarshal(v, &snap)
		if err != nil {
			return snapshots, err
		}
		snap.Hash = snap.PayloadHash()
		snap.TopologicalOrder = topology
		snapshots = append(snapshots, &snap)
	}

	return snapshots, nil
}

func (s *BadgerStore) TopologySequence() uint64 {
	var sequence uint64

	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(graphTopologyKey(^uint64(0)))
	if it.ValidForPrefix([]byte(graphPrefixTopology)) {
		item := it.Item()
		sequence = graphTopologyOrder(item.Key()) + 1
	}
	return sequence
}

func writeTopology(txn *badger.Txn, snap *common.SnapshotWithTopologicalOrder) error {
	key := graphTopologyKey(snap.TopologicalOrder)
	val := graphSnapshotKey(snap.NodeId, snap.RoundNumber, snap.Transaction)
	return txn.Set(key, val[:])
}

func graphTopologyKey(order uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, order)
	return append([]byte(graphPrefixTopology), buf...)
}

func graphTopologyOrder(key []byte) uint64 {
	order := key[len(graphPrefixTopology):]
	return binary.BigEndian.Uint64(order)
}
