package raft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

var (
	ErrNotLeader = fmt.Errorf("failed to get leader, assume first node in cluster")
)

type RestCoordinator struct {
	raft          *Raft
	log           *slog.Logger
	echo          *echo.Echo
	port          int
	grpcPort      int
	discoveryAddr string
	advertiseIP   string
	nodeID        string
}

type getLeaderResponse struct {
	LeaderID   string `json:"leader_id"`
	LeaderAddr string `json:"leader_address"`
}

type RestCoordinatorOpts struct {
	Logger *slog.Logger
	Echo   *echo.Echo
}

func NewRestCoordinator(opts RestCoordinatorOpts) *RestCoordinator {
	return &RestCoordinator{
		log:  opts.Logger,
		echo: opts.Echo,
	}
}

func (c *RestCoordinator) GetLeader() (string, string, error) {
	url := fmt.Sprintf("http://%s/raft/leader", c.discoveryAddr)
	c.log.Info("getting leader %s", "url", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.log.Error("failed to get leader", "error", err)
		return "", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.log.Error("failed to get leader", "error", err)
		return "", "", ErrNotLeader
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		c.log.Error("failed to get leader %s", "body", string(body))
		return "", "", fmt.Errorf("failed to get leader: %s", string(body))
	}

	data := getLeaderResponse{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		c.log.Error("failed to unmarshal leader response", "error", err)
		return "", "", err
	}
	c.log.Info("leader found %s", "leader ID", data.LeaderID)
	return data.LeaderAddr, data.LeaderID, nil
}

func (c *RestCoordinator) Join(address string, name string) error {
	leaderApi := strings.Replace(string(address), "9090", "8080", 1)
	c.log.Info("joining leader", "leader ID", name, "address", leaderApi)

	url := fmt.Sprintf("http://%s/raft/join", leaderApi)
	var jsonStr = []byte(fmt.Sprintf(`{"node_id":"%s","raft_address":"%s"}`, c.nodeID, fmt.Sprintf("%s:%d", c.advertiseIP, c.grpcPort)))

	fmt.Println(fmt.Sprintf("%s:%d", c.advertiseIP, c.grpcPort))
	fmt.Println(fmt.Sprintf("%s:%d", c.advertiseIP, c.grpcPort))
	fmt.Println(fmt.Sprintf("%s:%d", c.advertiseIP, c.grpcPort))
	fmt.Println(fmt.Sprintf("%s:%d", c.advertiseIP, c.grpcPort))
	fmt.Println(fmt.Sprintf("%s:%d", c.advertiseIP, c.grpcPort))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		c.log.Error("failed to join leader", "error", err)
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.log.Error("failed to join leader", "error", err)
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log.Error("failed to join leader", "error", err)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("failed to join leader %s", "body", string(body))
		return fmt.Errorf("failed to join leader: %s", string(body))
	}

	c.log.Info("joined leader successfully")
	return nil
}

func (c *RestCoordinator) Leave() error {
	address, leaderID := c.raft.LeaderWithID()
	leaderApi := strings.Replace(string(address), "9090", "8080", 1)

	c.log.Info("leaving leader %s", "leader ID", leaderID)

	url := fmt.Sprintf("http://%s/raft/remove/%s", leaderApi, c.nodeID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		c.log.Error("failed to leave leader", "error", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.log.Error("failed to leave leader", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.log.Error("failed to leave leader %s", "body", string(body))
		return err
	}

	c.log.Info("left leader successfully")

	err = c.raft.Shutdown()

	if err != nil {
		c.log.Error("failed to shutdown raft", "error", err)
		return err
	}

	c.log.Info("shutdown raft successfully")
	return nil
}

func (c *RestCoordinator) WithRaft(opts RaftOptions, raft *Raft) error {
	c.raft = raft
	c.port = opts.Port
	c.grpcPort = opts.GrpcPort
	c.discoveryAddr = opts.DiscoveryAddr
	c.advertiseIP = opts.AdvertiseIP
	c.nodeID = opts.NodeID

	RegisterCoordinatorRoutes(c.echo, raft.raft)
	return nil
}
