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

	//var pingerHost string
	//var pingerPort int
	//var systemDomain string
	//var loadBalancerPort int
	//
	//flag.StringVar(&pingerHost, "pingerHost", "", "Pinger Host")
	//flag.IntVar(&pingerPort, "pingerPort", 8000, "Pinger Port")
	//flag.StringVar(&systemDomain, "systemdomain", "", "system domain of the landscape")
	//flag.IntVar(&loadBalancerPort, "loadBalancerPort", 9000, "port of the load balancer of the landscape")

	var filename string
	flag.StringVar(&filename, "pingerConfigFile", "", "istio config file for pinger")

	flag.Parse()

	watcher, err := config.NewConfigWatcher(filename)
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
