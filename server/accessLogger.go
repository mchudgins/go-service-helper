package server

import (
	"go.uber.org/zap"
	xcontext "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func grpcEndpointLog(logger *zap.Logger, s string) grpc.UnaryServerInterceptor {
	return func(ctx xcontext.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {
		logger.Info("grpcEndpointLog+",
			zap.String("endpoint", s),
			zap.String("method", info.FullMethod))
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			for key, value := range md {
				logger.Info("metadata", zap.String(key, value[0]))
			}
		}
		defer func() {
			logger.Info("grpcEndpointLog-", zap.String("", s))
			logger.Sync()
		}()

		rc, err := handler(ctx, req)

		md, ok = metadata.FromOutgoingContext(ctx)
		if ok {
			for key, value := range md {
				logger.Info("outgoing metadata", zap.String(key, value[0]))
			}
		}

		return rc, err
	}
}
