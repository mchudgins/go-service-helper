package server

import (
	"expvar"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"context"

	afex "github.com/afex/hystrix-go/hystrix"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/justinas/alice"
	gsh "github.com/mchudgins/go-service-helper/handlers"
	"github.com/mchudgins/playground/pkg/healthz"
	"github.com/mwitkow/go-grpc-middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.uber.org/zap"
	xcontext "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Config struct {
	Insecure          bool
	CertFilename      string
	KeyFilename       string
	HTTPListenPort    int
	RPCListenPort     int
	MetricsListenPort int
	RPCRegister       RPCRegistration
	logger            *zap.Logger
	rpcServer         *grpc.Server
	httpServer        *http.Server
	metricsServer     *http.Server
}

type Option func(*Config) error

type RPCRegistration func(*grpc.Server) error

func WithRPCServer(fn RPCRegistration) Option {
	return func(cfg *Config) error {
		/*
			echoServer, err := NewServer(p.logger)
			if err != nil {
				cfg.logger.Panic("while creating new EchoServer", zap.Error(err))
			}
			rpc.RegisterEchoServiceServer(s, echoServer)
		*/
		cfg.RPCRegister = fn

		return nil
	}
}

func WithCertificate(certFilename, keyFilename string) Option {
	return func(cfg *Config) error {
		cfg.CertFilename = certFilename
		cfg.KeyFilename = keyFilename
		cfg.Insecure = false
		return nil
	}
}

func WithHTTPListenPort(port int) Option {
	return func(cfg *Config) error {
		cfg.HTTPListenPort = port
		return nil
	}
}

func WithRPCListenPort(port int) Option {
	return func(cfg *Config) error {
		cfg.RPCListenPort = port
		return nil
	}
}

func WithMetricsListenPort(port int) Option {
	return func(cfg *Config) error {
		cfg.MetricsListenPort = port
		return nil
	}
}

func WithLogger(l *zap.Logger) Option {
	return func(cfg *Config) error {
		cfg.logger = l
		return nil
	}
}

type sourcetype int

const (
	interrupt sourcetype = iota
	httpServer
	metricsServer
	rpcServer
)

type eventSource struct {
	source sourcetype
	err    error
}

func grpcEndpointLog(logger *zap.Logger, s string) grpc.UnaryServerInterceptor {
	return func(ctx xcontext.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {
		logger.Info("grpcEndpointLog+", zap.String("", s))
		defer func() {
			logger.Info("grpcEndpointLog-", zap.String("", s))
			logger.Sync()
		}()

		return handler(ctx, req)
	}
}

func Run(opts ...Option) {
	cfg := &Config{
		Insecure:          true,
		HTTPListenPort:    8443,
		MetricsListenPort: 8080,
		RPCListenPort:     50050,
	}

	for _, o := range opts {
		o(cfg)
	}

	// make a channel to listen on events,
	// then launch the servers.

	errc := make(chan eventSource)

	// interrupt handler
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- eventSource{
			source: interrupt,
			err:    fmt.Errorf("%s", <-c),
		}
	}()

	// gRPC server
	if cfg.RPCRegister != nil {
		go func() {
			rpcListenPort := ":" + strconv.Itoa(cfg.RPCListenPort)
			lis, err := net.Listen("tcp", rpcListenPort)
			if err != nil {
				errc <- eventSource{
					err:    err,
					source: rpcServer,
				}
				return
			}

			if cfg.Insecure {
				cfg.rpcServer = grpc.NewServer(
					grpc_middleware.WithUnaryServerChain(
						grpc_prometheus.UnaryServerInterceptor,
						grpcEndpointLog(cfg.logger, "certMgr")))
			} else {
				tlsCreds, err := credentials.NewServerTLSFromFile(cfg.CertFilename, cfg.KeyFilename)
				if err != nil {
					cfg.logger.Fatal("Failed to generate grpc TLS credentials", zap.Error(err))
				}
				cfg.rpcServer = grpc.NewServer(
					grpc.Creds(tlsCreds),
					grpc.RPCCompressor(grpc.NewGZIPCompressor()),
					grpc.RPCDecompressor(grpc.NewGZIPDecompressor()),
					grpc_middleware.WithUnaryServerChain(
						grpc_prometheus.UnaryServerInterceptor,
						grpcEndpointLog(cfg.logger, "Echo RPC server")))
			}

			cfg.RPCRegister(cfg.rpcServer)

			if cfg.Insecure {
				log.Warnf("gRPC service listening insecurely on %s", rpcListenPort)
			} else {
				log.Infof("gRPC service listening on %s", rpcListenPort)
			}
			errc <- eventSource{
				err:    cfg.rpcServer.Serve(lis),
				source: rpcServer,
			}
		}()
	}

	// health & metrics via https
	go func() {
		rootMux := mux.NewRouter() //actuator.NewActuatorMux("")

		hc, err := healthz.NewConfig()
		healthzHandler, err := healthz.Handler(hc)
		if err != nil {
			cfg.logger.Panic("Constructing healthz.Handler", zap.Error(err))
		}

		// set up handlers for THIS instance
		// (these are not expected to be proxied)
		rootMux.Handle("/debug/vars", expvar.Handler())
		rootMux.Handle("/healthz", healthzHandler)
		rootMux.Handle("/metrics", prometheus.Handler())

		canonical := handlers.CanonicalHost("http://fubar.local.dstcorp.io:7070", http.StatusPermanentRedirect)
		var tracer func(http.Handler) http.Handler
		tracer = gsh.TracerFromHTTPRequest(gsh.NewTracer("commandName"), "proxy")

		//rootMux.PathPrefix("/").Handler(p)

		chain := alice.New(tracer,
			gsh.HTTPMetricsCollector,
			gsh.HTTPLogrusLogger,
			canonical,
			handlers.CompressHandler)

		//errc <- http.ListenAndServe(p.address, chain.Then(rootMux))
		httpListenAddress := ":" + strconv.Itoa(cfg.HTTPListenPort)
		cfg.httpServer = &http.Server{
			Addr:              httpListenAddress,
			Handler:           chain.Then(rootMux),
			ReadTimeout:       time.Duration(5) * time.Second,
			ReadHeaderTimeout: time.Duration(2) * time.Second,
		}

		errc <- eventSource{
			err:    cfg.httpServer.ListenAndServeTLS(cfg.CertFilename, cfg.KeyFilename),
			source: httpServer,
		}
	}()

	// start the hystrix stream provider
	go func() {
		hystrixStreamHandler := afex.NewStreamHandler()
		hystrixStreamHandler.Start()
		listenPort := ":" + strconv.Itoa(cfg.MetricsListenPort)
		cfg.metricsServer = &http.Server{
			Addr:    listenPort,
			Handler: hystrixStreamHandler,
		}
		errc <- eventSource{
			err:    cfg.metricsServer.ListenAndServe(),
			source: metricsServer,
		}
	}()

	// wait for somthin'
	cfg.logger.Info("Echo Server",
		zap.Int("http port", cfg.HTTPListenPort),
		zap.Int("metrics port", cfg.MetricsListenPort),
		zap.Int("RPC port", cfg.RPCListenPort))
	rc := <-errc

	// somethin happened, now shut everything down gracefully
	cfg.logger.Info("exit", zap.Error(rc.err), zap.Int("source", int(rc.source)))
	waitDuration := time.Duration(5) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), waitDuration)
	defer cancel()

	waitEvents := 0
	evtc := make(chan eventSource)

	if rc.source != httpServer && cfg.httpServer != nil {
		waitEvents++
		go func() {
			evtc <- eventSource{
				err:    cfg.httpServer.Shutdown(ctx),
				source: httpServer,
			}
		}()
	}
	if rc.source != rpcServer && cfg.rpcServer != nil {
		waitEvents++
		go func() {
			cfg.rpcServer.GracefulStop()
			evtc <- eventSource{source: rpcServer}
		}()
	}
	if rc.source != metricsServer && cfg.metricsServer != nil {
		waitEvents++
		go func() {
			evtc <- eventSource{
				err:    cfg.metricsServer.Shutdown(ctx),
				source: metricsServer,
			}
		}()
	}

	// wait for shutdown to complete or time to expire
	for {
		select {
		case <-time.After(time.Duration(4) * time.Second):
			cfg.logger.Info("server shutdown complete")
			os.Exit(1)

		case <-ctx.Done():
			cfg.logger.Warn("shutdown time expired -- performing hard shutdown", zap.Error(ctx.Err()))
			os.Exit(2)

		case evt := <-evtc:
			waitEvents--
			cfg.logger.Info("waitEvent recv'ed", zap.Error(evt.err), zap.Int("eventSource", int(evt.source)))
			if waitEvents == 0 {
				os.Exit(0)
			}
		}
	}
}
