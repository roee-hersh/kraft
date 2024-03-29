package raft

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"paker/internal/raft/fsm"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/rs/zerolog/log"
)

type Coordinator interface {
	Join(id string, address string) error
	Leave() error
	GetLeader() (string, string, error)
	WithRaft(opts RaftOptions, raft *Raft) error
}

type Raft struct {
	raft          *raft.Raft
	transport     *raft.NetworkTransport
	log           *slog.Logger
	coordinator   Coordinator
	DiscoveryAddr string
	AdvertiseIP   string
	Port          int
	GrpcPort      int
	NodeID        string
	VolumeDir     string
}

const (
	// the timeout by (SnapshotSize / TimeoutScale).
	tcpTimeout = 10 * time.Second

	// The `retain` parameter controls how many
	// snapshots are retained. Must be at least 1.
	raftSnapShotRetain = 2

	// raftLogCacheSize is the maximum number of logs to cache in-memory.
	// This is used to reduce disk I/O for the recently committed entries.
	raftLogCacheSize  = 512
	maxPoolSize       = 3
	snapshotThreshold = 1024
)

func (rs *Raft) prepare() error {
	raftConf := raft.DefaultConfig()
	raftConf.LocalID = raft.ServerID(rs.NodeID)
	raftConf.SnapshotThreshold = snapshotThreshold

	// setup Raft backend storage
	fsmStore, err := fsm.NewBadgerFSM(rs.VolumeDir, rs.log)
	if err != nil {
		log.Error().Err(err).Msg("failed to open badger store")
		return err
	}

	// Setup Raft log store
	store, err := raftboltdb.NewBoltStore(filepath.Join(rs.VolumeDir, "raft.dataRepo"))
	if err != nil {
		log.Error().Err(err).Msg("failed to open bolt store")
		return err
	}

	// Wrap the store in a LogCache to improve performance.
	cacheStore, err := raft.NewLogCache(raftLogCacheSize, store)
	if err != nil {
		log.Error().Err(err).Msg("failed to open log cache")
		return err
	}

	snapshotStore, err := raft.NewFileSnapshotStore(rs.VolumeDir, raftSnapShotRetain, os.Stdout)
	if err != nil {
		log.Error().Err(err).Msg("failed to open snapshot store")
		return err
	}

	var raftBinAddr = fmt.Sprintf("%s:%d", rs.AdvertiseIP, rs.GrpcPort)

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", raftBinAddr)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(raftBinAddr, addr, maxPoolSize, tcpTimeout, os.Stderr)
	if err != nil {
		return err
	}

	raftServer, err := raft.NewRaft(raftConf, fsmStore, cacheStore, store, snapshotStore, transport)
	if err != nil {
		return err
	}

	rs.raft = raftServer
	rs.transport = transport

	return nil
}

func (rs *Raft) Start() error {

	// always start single server as a leader
	configuration := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(rs.NodeID),
				Address: rs.transport.LocalAddr(),
			},
		},
	}

	leaderAddr, leaderID, err := rs.coordinator.GetLeader()
	if err == nil {
		rs.log.Info("leader", "address", leaderAddr, "id", leaderID)
		// join cluster
		rs.raft.BootstrapCluster(configuration)
		rs.coordinator.Join(leaderAddr, leaderID)
	} else {
		rs.log.Info("leader not found, starting new cluster")
		fut := rs.raft.BootstrapCluster(configuration)
		if err := fut.Error(); err != nil {
			rs.log.Error("failed to bootstrap cluster", "error", fut.Error())
			return fut.Error()
		}
	}

	return nil
}

func (rs *Raft) Shutdown() error {
	if rs.raft != nil {
		fut := rs.raft.Shutdown()
		if err := fut.Error(); err != nil {
			rs.log.Error("failed to shutdown raft", "error", fut.Error())
			return err
		}
	}
	return nil
}

func (rs *Raft) LeaderWithID() (string, string) {
	leader := rs.raft.Leader()
	if leader == "" {
		return "", ""
	}
	address, id := rs.raft.LeaderWithID()
	return string(address), string(id)
}

type RaftOptions struct {
	DiscoveryAddr string
	AdvertiseIP   string
	Port          int
	GrpcPort      int
	NodeID        string
	VolumeDir     string
	Logger        *slog.Logger
	Coordinator   Coordinator
}

func NewRaft(opts RaftOptions) (*Raft, error) {
	raft := &Raft{
		coordinator:   opts.Coordinator,
		log:           opts.Logger,
		DiscoveryAddr: opts.DiscoveryAddr,
		AdvertiseIP:   opts.AdvertiseIP,
		Port:          opts.Port,
		GrpcPort:      opts.GrpcPort,
		NodeID:        opts.NodeID,
		VolumeDir:     opts.VolumeDir,
	}

	// create raft server with configuration but do not start it
	err := raft.prepare()

	// setup coordinator
	opts.Coordinator.WithRaft(opts, raft)

	return raft, err
}
