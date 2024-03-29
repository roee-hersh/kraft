package coordinator

import (
	"encoding/json"
	"fmt"
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

type NewTopicOptions struct {
	Name       string
	Partitions int
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

func (t *Coordinator) NewTopic(opts NewTopicOptions) error {
	// assign partitions to nodes
	cNode := 0
	partitions := make([]*Partition, 0)
	for i := 0; i < opts.Partitions; i++ {
		p := &Partition{
			ID:     i,
			Name:   fmt.Sprintf("%s-%d", opts.Name, i),
			NodeID: t.nodes[cNode].ID,
		}

		partitions = append(partitions, p)

		cNode++
		if cNode >= len(t.nodes) {
			cNode = 0
		}
	}

	topic := &Topic{
		Name:       opts.Name,
		Partitions: partitions,
	}

	command := &Command[Topic]{
		Op:   OpNewTopic,
		Data: topic,
	}

	cBody, err := json.Marshal(command)
	if err != nil {
		return err
	}

	fut := t.raft.Apply(cBody, NewTopicTimeout)

	if err := fut.Error(); err != nil {
		t.log.Error("failed to apply new topic command", "error", err)
		return err
	}

	t.log.Info("new topic applied", "topic", opts.Name)
	return nil
}

func (t *Coordinator) HandleNewTopic(c Command[Topic]) interface{} {
	// create partitions in storage
	for _, p := range c.Data.Partitions {
		// create partition
		if p.NodeID == t.nodeID {
			p := Partition{
				ID:     p.ID,
				Name:   p.Name,
				NodeID: p.NodeID,
			}
		}
	}
	return nil
}
