package server

import (
	"context"
	"os"
	"time"

	"go.uber.org/zap"
)

/*
	Handle the graceful shutdown of the server endpoints
*/

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

func (t sourcetype) String() string {
	sourcetypeNames := []string{"interrupt", "httpServer", "metricServer", "rpcServer"}

	return sourcetypeNames[t]
}

func (cfg *Config) performGracefulShutdown(ctx context.Context, evtSrc eventSource) {
	cfg.logger.Info("termination event detected", zap.Error(evtSrc.err), zap.String("source", evtSrc.source.String()))
	waitDuration := time.Duration(5) * time.Second
	ctx, cancel := context.WithTimeout(ctx, waitDuration)
	defer cancel()

	waitEvents := 0
	evtc := make(chan eventSource)
	defer close(evtc)

	if evtSrc.source != httpServer && cfg.httpServer != nil {
		waitEvents++
		go func() {
			evtc <- eventSource{
				err:    cfg.httpServer.Shutdown(ctx),
				source: httpServer,
			}
		}()
	}
	if evtSrc.source != rpcServer && cfg.rpcServer != nil {
		waitEvents++
		go func() {
			cfg.rpcServer.GracefulStop()
			evtc <- eventSource{source: rpcServer}
		}()
	}
	if evtSrc.source != metricsServer && cfg.metricsServer != nil {
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
			cfg.logger.Warn("wait time for service shutdown has elapsed -- performing hard shutdown", zap.Error(ctx.Err()))
			os.Exit(2)

		case evt := <-evtc:
			waitEvents--
			cfg.logger.Info("shutdown event recv'ed", zap.Error(evt.err), zap.String("eventSource", evt.source.String()))
			if waitEvents == 0 {
				os.Exit(0)
			}
		}
	}
}
