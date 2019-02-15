package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"github.com/Peripli/service-manager-istio-mpc-server/pkg/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io/ioutil"
	"istio.io/api/mcp/v1alpha1"
	"istio.io/istio/pkg/mcp/monitoring"
	"istio.io/istio/pkg/mcp/server"
	"istio.io/istio/pkg/mcp/source"
	"log"
	"net"
)

func main() {

	var configDir string
	var tlsMode string
	flag.StringVar(&configDir, "configDir", "", "istio config directory")
	flag.StringVar(&tlsMode, "tlsMode", "MUTUAL", "tls mode. Possible values: NONE, MUTUAL.")

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
	switch tlsMode {
	case "MUTUAL":
		grpcOptions = append(grpcOptions, grpc.Creds(credentials.NewTLS(tlsConfig())))
	case "NONE":
	default:
		log.Panic(fmt.Sprintf("Invalid TLS mode %s", tlsMode))
	}
	grpcOptions = append(grpcOptions, grpc.MaxConcurrentStreams(1024))
	grpcOptions = append(grpcOptions, grpc.MaxRecvMsgSize(1024*1024))
	grpcServer := grpc.NewServer(grpcOptions...)

	log.Println("Setting up tls config")

	v1alpha1.RegisterAggregatedMeshConfigServiceServer(grpcServer, mcpServer)

	grpcListener, err := net.Listen("tcp", ":18000")
	if err != nil {
		panic(err)
	}

	err = grpcServer.Serve(grpcListener)
	if err != nil {
		panic(err)
	}

}

func tlsConfig() *tls.Config {
	serverCert, err := tls.LoadX509KeyPair("config/certs/mcp.crt", "config/certs/mcp.key")
	if err != nil {
		panic(err)
	}
	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile("config/certs/ca.crt")
	if err != nil {
		panic(fmt.Errorf("could not read ca-file: %s", err))
	}
	ok := certPool.AppendCertsFromPEM(ca)
	if !ok {
		panic("Could not append ca cert to cert pool.")
	}
	return &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		Certificates: []tls.Certificate{serverCert},
	}
}
