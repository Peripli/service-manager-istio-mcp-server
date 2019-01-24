package main

import (
	"flag"
	"github.com/Peripli/service-manager-istio-mpc-server/pkg/config"
	"google.golang.org/grpc"
	"istio.io/api/mcp/v1alpha1"
	"istio.io/istio/pkg/mcp/monitoring"
	"istio.io/istio/pkg/mcp/server"
	"istio.io/istio/pkg/mcp/source"
	"net"
)

func main() {

	var configDir string
	flag.StringVar(&configDir, "configDir", "", "istio config directory")

	flag.Parse()

	watcher, err := config.NewConfigWatcher(configDir)
	if err != nil {
		panic(err)
	}
	mcpServer := server.New(&source.Options{
		Watcher:            watcher,
		Reporter:           monitoring.NewStatsContext("mcp"),
		CollectionsOptions: config.Collections()}, &server.AllowAllChecker{})

	var grpcOptions []grpc.ServerOption
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(1024))
	grpcOptions = append(grpcOptions, grpc.MaxRecvMsgSize(1024*1024))
	grpcServer := grpc.NewServer(grpcOptions...)

	v1alpha1.RegisterAggregatedMeshConfigServiceServer(grpcServer, mcpServer)

	grpcListener, err := net.Listen("tcp", ":18000")
	if err != nil {
		panic(err)
	}
	grpcServer.Serve(grpcListener)

}
