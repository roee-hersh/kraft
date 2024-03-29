package coordinator

import (
	"log/slog"
	"time"

	"github.com/hashicorp/raft"
)

type Coordinator struct {
	nodeID string
	raft   *raft.Raft
	topics []*Topic
	nodes  []*Node
	log    *slog.Logger
}

const (
	OpNewTopic      = "new-topic"
	NewTopicTimeout = time.Second * 10
)

type Command[T any] struct {
	Op   string
	Data *T
}

type Topic struct {
	Name       string       `json:"name"`
	Partitions []*Partition `json:"partitions"`
}

type Node struct {
	ID   string
	Addr string
}

type Partition struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	NodeID string `json:"node_id"`
}
