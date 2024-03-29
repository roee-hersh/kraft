package raft

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/raft"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

type R map[string]interface{}
type Handlers struct {
	raft *raft.Raft
}

func NewHandlers(raft *raft.Raft) *Handlers {
	return &Handlers{
		raft: raft,
	}
}

func (r *Handlers) RaftStatsHandler(c echo.Context) error {
	address, id := r.raft.LeaderWithID()
	return c.JSON(http.StatusOK, R{
		"message":        "raft stats",
		"leader_id":      id,
		"leader_address": address,
		"stats":          r.raft.Stats(),
	})
}

func (r *Handlers) RemoveRaftHandler(c echo.Context) error {
	var nodeID = c.Param("node_id")

	if nodeID == "" {
		log.Error().Msg("node_id is required")
		return c.JSON(http.StatusUnprocessableEntity, R{
			"error": "node_id is required",
		})
	}
	if r.raft.State() != raft.Leader {
		log.Error().Msg("not the leader")
		return c.JSON(http.StatusUnprocessableEntity, R{
			"error": "not the leader",
		})
	}

	configFuture := r.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Error().Err(err).Msgf("failed to get raft configuration")
		return c.JSON(http.StatusUnprocessableEntity, R{
			"error": fmt.Sprintf("failed to get raft configuration: %s", err.Error()),
		})
	}

	future := r.raft.RemoveServer(raft.ServerID(nodeID), 0, 0)
	if err := future.Error(); err != nil {
		log.Error().Err(err).Msgf("error removing existing node %s", nodeID)
		return c.JSON(http.StatusUnprocessableEntity, R{
			"error": fmt.Sprintf("error removing existing node %s: %s", nodeID, err.Error()),
		})
	}

	return c.JSON(http.StatusOK, R{
		"message": fmt.Sprintf("node %s  removed successfully", nodeID),
		"data":    r.raft.Stats(),
	})
}

// requestJoin request payload for joining raft cluster
type JoinRaftRequest struct {
	NodeID      string `json:"node_id"`
	RaftAddress string `json:"raft_address"`
}

func (r *Handlers) JoinRaftHandler(c echo.Context) error {
	var form = JoinRaftRequest{}
	if err := c.Bind(&form); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, R{
			"error": fmt.Sprintf("error binding: %s", err.Error()),
		})
	}

	var (
		nodeID   = form.NodeID
		raftAddr = form.RaftAddress
	)

	configFuture := r.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, R{
			"error": fmt.Sprintf("failed to get raft configuration: %s", err.Error()),
		})
	}

	// This must be run on the leader or it will fail.
	f := r.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(raftAddr), 0, 0)
	if f.Error() != nil {
		return c.JSON(http.StatusUnprocessableEntity, R{
			"error": fmt.Sprintf("error add voter: %s", f.Error().Error()),
		})
	}

	return c.JSON(http.StatusOK, R{
		"message": fmt.Sprintf("node %s at %s joined successfully", nodeID, raftAddr),
		"data":    r.raft.Stats(),
	})
}

func (r *Handlers) RaftLeaderHandler(c echo.Context) error {
	if r.raft == nil {
		return c.JSON(http.StatusServiceUnavailable, R{
			"message": "raft is not ready",
		})
	}

	address, id := r.raft.LeaderWithID()
	if id == "" {
		return c.JSON(http.StatusServiceUnavailable, R{
			"message": "no leader",
		})
	}
	return c.JSON(http.StatusOK, R{
		"message":        "raft leader",
		"leader_id":      id,
		"leader_address": address,
	})
}

func RegisterCoordinatorRoutes(e *echo.Echo, raft *raft.Raft) {
	handlers := NewHandlers(raft)
	raftApi := e.Group("/raft")
	raftApi.POST("/join", handlers.JoinRaftHandler)
	raftApi.POST("/remove/:nodeID", handlers.RemoveRaftHandler)
	raftApi.GET("/leader", handlers.RaftLeaderHandler)
	raftApi.GET("/stats", handlers.RaftStatsHandler)
}
