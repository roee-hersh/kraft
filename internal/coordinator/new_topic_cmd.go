package coordinator

import (
	"encoding/json"
	"fmt"
)

type NewTopicOptions struct {
	Name       string
	Partitions int
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
