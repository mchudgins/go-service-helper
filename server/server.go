package server

import (
	"context"
	"expvar"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	afex "github.com/afex/hystrix-go/hystrix"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/justinas/alice"
	gsh "github.com/mchudgins/go-service-helper/handlers"
	"github.com/mchudgins/playground/pkg/healthz"
	"github.com/mwitkow/go-grpc-middleware"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	xcontext "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Config struct {
	Insecure          bool
	Compress          bool // if true, add compression handling to messages
	CertFilename      string
	KeyFilename       string
	HTTPListenPort    int
	RPCListenPort     int
	MetricsListenPort int
	Handler           http.Handler
	Hostname          string // if present, enforce canonical hostnames
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
		cfg.RPCRegister = fn

		return nil
	}
}

func WithHTTPServer(h http.Handler) Option {
	return func(cfg *Config) error {
		cfg.Handler = h
		return nil
	}
}

func WithCanonicalHost(hostname string) Option {
	return func(cfg *Config) error {
		cfg.Hostname = hostname

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

func Run(ctx context.Context, opts ...Option) {

	// default config
	cfg := &Config{
		Insecure:          true,
		HTTPListenPort:    8443,
		MetricsListenPort: 8080,
		RPCListenPort:     50050,
	}

	// process the Run() options
	for _, o := range opts {
		o(cfg)
	}

	// make a channel to listen on events,
	// then launch the servers.

	errc := make(chan eventSource)
	defer close(errc)

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

			errc <- eventSource{
				err:    cfg.rpcServer.Serve(lis),
				source: rpcServer,
			}
		}()
	}

	// http/https server
	if cfg.Handler != nil {
		go func() {
			rootMux := mux.NewRouter()

			hc, err := healthz.NewConfig()
			healthzHandler, err := healthz.Handler(hc)
			if err != nil {
				cfg.logger.Panic("Constructing healthz.Handler", zap.Error(err))
			}

			// TODO: move these three handlers to the metrics listener
			// set up handlers for THIS instance
			// (these are not expected to be proxied)
			rootMux.Handle("/debug/vars", expvar.Handler())
			rootMux.Handle("/healthz", healthzHandler)
			rootMux.Handle("/metrics", prometheus.Handler())

			rootMux.PathPrefix("/").Handler(cfg.Handler)

			var tracer func(http.Handler) http.Handler
			tracer = gsh.TracerFromHTTPRequest(gsh.NewTracer("commandName"), "proxy")

			chain := alice.New(tracer,
				gsh.HTTPMetricsCollector,
				gsh.HTTPLogrusLogger)

			if len(cfg.Hostname) > 0 {
				canonical := handlers.CanonicalHost(cfg.Hostname, http.StatusPermanentRedirect)
				chain = chain.Append(canonical)
			}

			if cfg.Compress {
				chain = chain.Append(handlers.CompressHandler)
			}

			httpListenAddress := ":" + strconv.Itoa(cfg.HTTPListenPort)
			cfg.httpServer = &http.Server{
				Addr:              httpListenAddress,
				Handler:           chain.Then(rootMux),
				ReadTimeout:       time.Duration(5) * time.Second,
				ReadHeaderTimeout: time.Duration(2) * time.Second,
				TLSConfig:         tlsConfigFactory(),
			}

			if cfg.Insecure {
				err = cfg.httpServer.ListenAndServe()
			} else {
				err = cfg.httpServer.ListenAndServeTLS(cfg.CertFilename, cfg.KeyFilename)
			}

			errc <- eventSource{
				err:    err,
				source: httpServer,
			}
		}()
	}

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

	cfg.logLaunch()

	// wait for somthin'
	rc := <-errc

	// somethin happened, now shut everything down gracefully, if possible
	cfg.performGracefulShutdown(ctx, rc)
}

func (cfg *Config) logLaunch() {
	serverList := make([]zapcore.Field, 0, 5)

	if cfg.RPCRegister != nil {
		serverList = append(serverList, zap.Int("gRPC port", cfg.RPCListenPort))
	}
	if cfg.Handler != nil {
		var key = "HTTPS port"
		if cfg.Insecure {
			key = "HTTP port"
		}
		serverList = append(serverList, zap.Int(key, cfg.HTTPListenPort))
	}
	serverList = append(serverList, zap.Int("metrics port", cfg.MetricsListenPort))

	if cfg.Insecure {
		cfg.logger.Warn("Server listening insecurely on one or more ports", serverList...)
	} else {
		cfg.logger.Info("Server", serverList...)
	}
}
