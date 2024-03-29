package api

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/labstack/echo/v4"
)

var gracefulShutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGINT}

type Server struct {
	listenAddr      string
	port            int
	shutdownSignals []os.Signal
	echo            *echo.Echo
	log             *slog.Logger
}

type ServerOptions struct {
	Echo            *echo.Echo
	Logger          *slog.Logger
	ShutdownSignals []os.Signal
	ListenAddr      string
	Port            int
}

func NewServer(opts ServerOptions) *Server {

	s := &Server{
		listenAddr:      opts.ListenAddr,
		port:            opts.Port,
		shutdownSignals: gracefulShutdownSignals,
		echo:            opts.Echo,
		log:             opts.Logger,
	}

	return s
}

func (s *Server) listen() {
	go func() {
		if err := s.echo.Start(fmt.Sprintf("%s:%d", s.listenAddr, s.port)); err != http.ErrServerClosed {
			log.Fatal(err)
		}

		fmt.Printf("Server started at %s:%d", s.listenAddr, s.port)
	}()
}

func (s *Server) createTerminationSignal() (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	return ctx, stop
}

func (s *Server) gracefulShutdown(ctx context.Context) {
	// Wait for interrupt signal to gracefully shut down the server with a timeout of 10 seconds.
	log.Println("received SIGINT or SIGTERM, gracefully shutting down the server")

	// Shutdown the server
	if err := s.echo.Shutdown(ctx); err != nil {
		log.Fatal("failed to shutdown http server", err)
	}
}

func (s *Server) Start() {
	ctx, stop := s.createTerminationSignal()
	defer stop()

	s.log.Info("server starting", "address", s.listenAddr, "port", s.port)

	// Start the server in a goroutine
	s.listen()

	s.log.Info("server started", "address", s.listenAddr, "port", s.port)
	// Wait for interrupt signal
	<-ctx.Done()

	s.log.Info("server stopping", "address", s.listenAddr, "port", s.port)
	// Shutdown the server
	s.gracefulShutdown(ctx)
}
