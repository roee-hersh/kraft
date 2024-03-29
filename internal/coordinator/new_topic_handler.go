package coordinator

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
