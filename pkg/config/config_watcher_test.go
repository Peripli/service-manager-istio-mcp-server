package config

import (
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	. "github.com/onsi/gomega"
	"io/ioutil"
	mcp "istio.io/api/mcp/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/galley/pkg/metadata"
	"istio.io/istio/pkg/mcp/snapshot"
	"istio.io/istio/pkg/mcp/source"
	"os"
	"path"
	"testing"
)

func TestCollections(t *testing.T) {
	g := NewGomegaWithT(t)
	g.Expect(Collections()).To(ContainElement(source.CollectionOptions{Name: "type.googleapis.com/istio.mixer.v1.config.client.QuotaSpecBinding"}))
}

func readSnapshotFromFile(filename string) (snapshot.Snapshot, error) {
	configs := make(map[string][]namedSpec)
	err := readConfigMapFromFile(filename, configs)
	if err != nil {
		return nil, err
	}
	return configMapToSnapshot(configs, 1)
}

func TestReadSnapshotFromFile(t *testing.T) {
	g := NewGomegaWithT(t)
	snapshot, err := readSnapshotFromFile("../../test/config/istio-pinger.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	serviceEntries := snapshot.Resources(metadata.ServiceEntry.TypeURL.String())
	g.Expect(serviceEntries).To(HaveLen(1))
	g.Expect(serviceEntries[0].Metadata.Name).To(Equal("pinger"))
	serviceEntry := &networking.ServiceEntry{}
	unWrapResource(serviceEntries[0], serviceEntry)
	g.Expect(serviceEntry.Hosts).To(HaveLen(1))
	g.Expect(serviceEntry.Hosts[0]).To(Equal("istio-pinger.istio"))

	virtualServices := snapshot.Resources(metadata.VirtualService.TypeURL.String())
	g.Expect(virtualServices).To(HaveLen(1))
	g.Expect(virtualServices[0].Metadata.Name).To(Equal("pinger"))
	virtualService := &networking.VirtualService{}
	unWrapResource(virtualServices[0], virtualService)
	g.Expect(virtualService.Hosts).To(HaveLen(1))
	g.Expect(virtualService.Hosts[0]).To(Equal("pinger.istio.cf.dev01.aws.istio.sapcloud.io"))
	g.Expect(virtualService.Tcp[0].Route[0].Destination.Host).To(Equal("istio-pinger.istio"))
	g.Expect(virtualService.Tcp[0].Route[0].Destination.Port.GetNumber()).To(Equal(uint32(8081)))

	gateways := snapshot.Resources(metadata.Gateway.TypeURL.String())
	g.Expect(gateways).To(HaveLen(1))
	g.Expect(gateways[0].Metadata.Name).To(Equal("pinger-gateway"))
	gateway := &networking.Gateway{}
	unWrapResource(gateways[0], gateway)
	g.Expect(gateway.Servers[0].Hosts[0]).To(Equal("pinger.istio.cf.dev01.aws.istio.sapcloud.io"))
	g.Expect(gateway.Servers[0].Port.Number).To(Equal(uint32(9000)))

}

func TestReadSnapshotFromInvalidFile(t *testing.T) {
	g := NewGomegaWithT(t)
	_, err := readSnapshotFromFile("../../test/config/front-envoy.yaml")
	g.Expect(err).To(HaveOccurred())

}

func TestReadSnapshotFromDirectory(t *testing.T) {
	g := NewGomegaWithT(t)
	snapshot, err := readSnapshotFromDirectory("../../test/config", 1)
	g.Expect(err).NotTo(HaveOccurred())
	serviceEntries := snapshot.Resources(metadata.ServiceEntry.TypeURL.String())
	g.Expect(serviceEntries).To(HaveLen(2))
	g.Expect(serviceEntries[0].Metadata.Name).To(Or(Equal("pinger"), Equal("test")))
	g.Expect(serviceEntries[1].Metadata.Name).To(Or(Equal("pinger"), Equal("test")))
	serviceEntry := &networking.ServiceEntry{}
	unWrapResource(serviceEntries[0], serviceEntry)
	g.Expect(serviceEntry.Hosts).To(HaveLen(1))
	g.Expect(serviceEntry.Hosts[0]).To(Or(Equal("istio-pinger.istio"), Equal("istio-test.istio")))
	unWrapResource(serviceEntries[1], serviceEntry)
	g.Expect(serviceEntry.Hosts).To(HaveLen(1))
	g.Expect(serviceEntry.Hosts[0]).To(Or(Equal("istio-pinger.istio"), Equal("istio-test.istio")))

	virtualServices := snapshot.Resources(metadata.VirtualService.TypeURL.String())
	g.Expect(virtualServices).To(HaveLen(2))
	g.Expect(virtualServices[0].Metadata.Name).To(Or(Equal("pinger"), Equal("test")))
	g.Expect(virtualServices[1].Metadata.Name).To(Or(Equal("pinger"), Equal("test")))

	gateways := snapshot.Resources(metadata.Gateway.TypeURL.String())
	g.Expect(gateways).To(HaveLen(2))

}

func TestConfigWatcher(t *testing.T) {
	g := NewGomegaWithT(t)
	dir, err := ioutil.TempDir("", "service-manager-istio-mcp-server")
	g.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(dir)

	configWatcher, err := NewConfigWatcher(dir)
	g.Expect(err).NotTo(HaveOccurred())
	defer configWatcher.Stop()

	channel := make(chan *source.WatchResponse, 10)
	callback := func(response *source.WatchResponse) {
		channel <- response
	}
	var response *source.WatchResponse
	// add one file
	{
		configWatcher.Watch(&source.Request{Collection: metadata.ServiceEntry.TypeURL.String()}, callback)

		err = os.Link("../../test/config/istio-pinger.yaml", path.Join(dir, "istio-pinger.yaml"))
		g.Expect(err).NotTo(HaveOccurred())

		response = <-channel
		serviceEntries := response.Resources

		g.Expect(serviceEntries).To(HaveLen(1))
		g.Expect(serviceEntries[0].Metadata.Name).To(Equal("pinger"))
		serviceEntry := &networking.ServiceEntry{}
		unWrapResource(serviceEntries[0], serviceEntry)
		g.Expect(serviceEntry.Hosts).To(HaveLen(1))
		g.Expect(serviceEntry.Hosts[0]).To(Equal("istio-pinger.istio"))
	}
	// add second file
	{
		configWatcher.Watch(&source.Request{Collection: metadata.ServiceEntry.TypeURL.String(), VersionInfo: response.Version}, func(response *source.WatchResponse) {
			channel <- response
		})

		err = os.Link("../../test/config/sub/istio-test.yaml", path.Join(dir, "istio-test.yaml"))
		g.Expect(err).NotTo(HaveOccurred())

		response = <-channel
		serviceEntries := response.Resources

		g.Expect(serviceEntries).To(HaveLen(2))
	}
	// remove first file
	{
		configWatcher.Watch(&source.Request{Collection: metadata.ServiceEntry.TypeURL.String(), VersionInfo: response.Version}, func(response *source.WatchResponse) {
			channel <- response
		})
		err = os.Remove(path.Join(dir, "istio-pinger.yaml"))
		g.Expect(err).NotTo(HaveOccurred())
		response = <-channel
		serviceEntries := response.Resources

		g.Expect(serviceEntries).To(HaveLen(1))
		g.Expect(serviceEntries[0].Metadata.Name).To(Equal("test"))
		serviceEntry := &networking.ServiceEntry{}
		unWrapResource(serviceEntries[0], serviceEntry)
		g.Expect(serviceEntry.Hosts).To(HaveLen(1))
		g.Expect(serviceEntry.Hosts[0]).To(Equal("istio-test.istio"))
	}
	// remove second file
	{
		configWatcher.Watch(&source.Request{Collection: metadata.ServiceEntry.TypeURL.String(), VersionInfo: response.Version}, func(response *source.WatchResponse) {
			channel <- response
		})
		err = os.Remove(path.Join(dir, "istio-test.yaml"))
		g.Expect(err).NotTo(HaveOccurred())
		response = <-channel
		serviceEntries := response.Resources

		g.Expect(serviceEntries).To(HaveLen(0))
	}

}

func unWrapResource(resource *mcp.Resource, message proto.Message) error {
	return types.UnmarshalAny(resource.Body, message)
}
