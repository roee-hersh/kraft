package config

import (
	"fmt"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

type Config struct {
	DiscoveryIP string `json:"discovery_ip" env:"DISCOVERY_IP" envDefault:"127.0.0.1:8080"`
	AdvertiseIP string `json:"advertise_ip" env:"ADVERTISE_IP" envDefault:"127.0.0.1"`
	Port        int    `json:"port" env:"PORT" envDefault:"8080"`
	ListenAddr  string `json:"listen_addr" env:"LISTEN_ADDR" envDefault:"127.0.0.1"`
	GrpcPort    int    `json:"grpc_port" env:"GRPC_PORT" envDefault:"9090"`
	NodeID      string `json:"node_id" env:"NODE_ID" envDefault:"node0"`
	VolumeDir   string `json:"volume_dir" env:"VOLUME_DIR"`
}

type OptFunc func(*Config)

func WithDiscoveryIP(ip string) OptFunc {
	return func(c *Config) {
		c.DiscoveryIP = ip
	}
}

func WithAdvertiseIP(ip string) OptFunc {
	return func(c *Config) {
		c.AdvertiseIP = ip
	}
}

func WithPort(port int) OptFunc {
	return func(c *Config) {
		c.Port = port
	}
}

func WithGrpcPort(port int) OptFunc {
	return func(c *Config) {
		c.GrpcPort = port
	}
}

func WithNodeID(id string) OptFunc {
	return func(c *Config) {
		c.NodeID = id
	}
}

func WithDefaults(c *Config) {
	if c.VolumeDir == "" {
		c.VolumeDir = "tmp/" + c.NodeID
	}
}

func NewConfig(opts ...OptFunc) *Config {
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}

	c := Config{}
	if err := env.Parse(&c); err != nil {
		fmt.Printf("%+v\n", err)
	}
	for _, opt := range opts {
		opt(&c)
	}
	return &c
}
