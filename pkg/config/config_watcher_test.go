package config

import (
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	. "github.com/onsi/gomega"
	mcp "istio.io/api/mcp/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/galley/pkg/metadata"
	"istio.io/istio/pkg/mcp/source"
	"testing"
)

func TestNewConfigWatcher(t *testing.T) {
	g := NewGomegaWithT(t)

	configWatcher, err := NewConfigWatcherInMem("192.0.0.1", 8000, "istio.cf.dev01.aws.istio.sapcloud.io", 9000)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(configWatcher).NotTo(BeNil())
}

func TestGetSnapshot(t *testing.T) {
	g := NewGomegaWithT(t)

	configWatcher, err := NewConfigWatcherInMem("192.0.0.1", 8000, "istio.cf.dev01.aws.istio.sapcloud.io", 9000)
	snapshot, err := configWatcher.getSnapshot()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(snapshot).NotTo(BeNil())
	serviceEntries := snapshot.Resources(metadata.ServiceEntry.TypeURL.String())
	g.Expect(serviceEntries).To(HaveLen(1))
	g.Expect(serviceEntries[0].Metadata.Name).To(Equal("pinger-service-entry"))
	serviceEntry := &networking.ServiceEntry{}
	unWrapResource(serviceEntries[0], serviceEntry)
	g.Expect(serviceEntry.Hosts).To(HaveLen(1))
	g.Expect(serviceEntry.Hosts[0]).To(Equal("pinger.service-fabrik"))

	virtualServices := snapshot.Resources(metadata.VirtualService.TypeURL.String())
	g.Expect(virtualServices).To(HaveLen(1))
	g.Expect(virtualServices[0].Metadata.Name).To(Equal("pinger-virtual-service"))
	virtualService := &networking.VirtualService{}
	unWrapResource(virtualServices[0], virtualService)
	g.Expect(virtualService.Hosts).To(HaveLen(1))
	g.Expect(virtualService.Hosts[0]).To(Equal("pinger.istio.cf.dev01.aws.istio.sapcloud.io"))
	g.Expect(virtualService.Tcp[0].Route[0].Destination.Host).To(Equal("pinger.service-fabrik"))
	g.Expect(virtualService.Tcp[0].Route[0].Destination.Port.GetNumber()).To(Equal(uint32(8000)))

	gateways := snapshot.Resources(metadata.Gateway.TypeURL.String())
	g.Expect(gateways).To(HaveLen(1))
	g.Expect(gateways[0].Metadata.Name).To(Equal("pinger-gateway"))
	gateway := &networking.Gateway{}
	unWrapResource(gateways[0], gateway)
	g.Expect(gateway.Servers[0].Hosts[0]).To(Equal("pinger.istio.cf.dev01.aws.istio.sapcloud.io"))
	g.Expect(gateway.Servers[0].Port.Number).To(Equal(uint32(9000)))

}

func TestCollections(t *testing.T) {
	g := NewGomegaWithT(t)
	g.Expect(Collections()).To(ContainElement(source.CollectionOptions{Name: "type.googleapis.com/istio.mixer.v1.config.client.QuotaSpecBinding"}))
}

func TestReadSnapshotFromFile(t *testing.T) {
	g := NewGomegaWithT(t)
	filename := "../../test/config/istio-pinger.yaml"
	configWatcher, err := NewConfigWatcher(filename)
	snapshot, err := configWatcher.readSnapshotFromFile()
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
	filename := "../../test/config/front-envoy.yaml"
	_, err := NewConfigWatcher(filename)
	g.Expect(err).To(HaveOccurred())

}

func TestReadSnapshotFromDirectory(t *testing.T) {
	g := NewGomegaWithT(t)
	snapshot, err := readSnapshotFromDirectory("../../test/config")
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

func unWrapResource(resource *mcp.Resource, message proto.Message) error {
	return types.UnmarshalAny(resource.Body, message)
}
