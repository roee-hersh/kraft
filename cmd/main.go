package main

import (
	"log/slog"
	"os"
	"paker/internal/api"
	"paker/internal/raft"
	"paker/util/config"

	"github.com/labstack/echo/v4"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	conf := config.NewConfig(config.WithDefaults)
	e := echo.New()
	e.HideBanner = true

	// rest coordinator for raft nodes to communicate
	coordinator := raft.NewRestCoordinator(raft.RestCoordinatorOpts{
		Logger: logger,
		Echo:   e,
	})

	// create new raft instance
	raft, err := raft.NewRaft(raft.RaftOptions{
		Coordinator:   coordinator,
		Logger:        logger,
		DiscoveryAddr: conf.DiscoveryIP,
		AdvertiseIP:   conf.AdvertiseIP,
		Port:          conf.Port,
		GrpcPort:      conf.GrpcPort,
		NodeID:        conf.NodeID,
		VolumeDir:     conf.VolumeDir,
	})

	if err != nil {
		logger.Error("error creating raft", "error", err.Error())
		return
	}

	// create new server instance
	server := api.NewServer(api.ServerOptions{
		Echo:       e,
		Logger:     logger,
		ListenAddr: conf.ListenAddr,
		Port:       conf.Port,
	})

	raft.Start()

	server.Start()
}
