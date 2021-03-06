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
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/justinas/alice"
	"github.com/mchudgins/go-service-helper/correlationID"
	gsh "github.com/mchudgins/go-service-helper/handlers"
	"github.com/mchudgins/playground/pkg/healthz"
	"github.com/mwitkow/go-grpc-middleware"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Config struct {
	Insecure          bool
	Compress          bool // if true, add compression handling to messages
	UseZipkin         bool // if true, add zipkin tracing
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
	serviceName       string
}

type Option func(*Config) error

type RPCRegistration func(*grpc.Server) error

const (
	zipkinHTTPEndpoint = "http://localhost:9411/api/v1/spans"
)

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

func WithHTTPServer(h http.Handler) Option {
	return func(cfg *Config) error {
		cfg.Handler = h
		return nil
	}
}

func WithLogger(l *zap.Logger) Option {
	return func(cfg *Config) error {
		cfg.logger = l
		return nil
	}
}

func WithMetricsListenPort(port int) Option {
	return func(cfg *Config) error {
		cfg.MetricsListenPort = port
		return nil
	}
}

func WithRPCListenPort(port int) Option {
	return func(cfg *Config) error {
		cfg.RPCListenPort = port
		return nil
	}
}

func WithRPCServer(fn RPCRegistration) Option {
	return func(cfg *Config) error {
		cfg.RPCRegister = fn

		return nil
	}
}

func WithServiceName(serviceName string) Option {
	return func(cfg *Config) error {
		cfg.serviceName = serviceName
		return nil
	}
}

func WithZipkinTracer() Option {
	return func(cfg *Config) error {
		cfg.UseZipkin = true
		return nil
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

			// configure the RPC server
			var grpcMiddleware grpc.ServerOption
			if cfg.UseZipkin {
				grpcMiddleware = grpc_middleware.WithUnaryServerChain(
					grpc_prometheus.UnaryServerInterceptor,
					otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer(), otgrpc.LogPayloads()),
					grpcEndpointLog(cfg.logger, cfg.serviceName))
			} else {
				grpcMiddleware = grpc_middleware.WithUnaryServerChain(
					grpc_prometheus.UnaryServerInterceptor,
					grpcEndpointLog(cfg.logger, cfg.serviceName))
			}

			if cfg.Insecure {
				cfg.rpcServer = grpc.NewServer(
					grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
					grpcMiddleware)
			} else {
				tlsCreds, err := credentials.NewServerTLSFromFile(cfg.CertFilename, cfg.KeyFilename)
				if err != nil {
					cfg.logger.Fatal("Failed to generate grpc TLS credentials", zap.Error(err))
				}
				cfg.rpcServer = grpc.NewServer(
					grpc.Creds(tlsCreds),
					grpc.RPCCompressor(grpc.NewGZIPCompressor()),
					grpc.RPCDecompressor(grpc.NewGZIPDecompressor()),
					grpcMiddleware)
			}

			cfg.RPCRegister(cfg.rpcServer)

			// register w. prometheus
			grpc_prometheus.Register(cfg.rpcServer)
			grpc_prometheus.EnableHandlingTimeHistogram()

			// run the server & send an event upon termination
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

			chain := alice.New(gsh.HTTPMetricsCollector, gsh.HTTPLogrusLogger)

			if cfg.UseZipkin {
				var tracer func(http.Handler) http.Handler
				tracer = gsh.TracerFromHTTPRequest(gsh.NewTracer("commandName"), "proxy")
				chain = chain.Append(tracer)
			} else {
				chain = chain.Append(func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

						// tag this request with a correlation ID, so we can troubleshoot it later, if necessary
						req, corrID, fExisted := correlationID.FromRequest(req)

						// if we're at the edge of the system, send the correlation ID back in the response
						if !fExisted {
							w.Header().Set(correlationID.CORRID, corrID)
						}

						next.ServeHTTP(w, req)
					})

				})
			}

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
	serverList := make([]zapcore.Field, 0, 10)

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
		cfg.logger.Info("Server listening insecurely on one or more ports", serverList...)
	} else {
		cfg.logger.Info("Server", serverList...)
	}
}
