package config

import (
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	. "github.com/onsi/gomega"
	mcp "istio.io/api/mcp/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
	"testing"
)

func TestNewConfigWatcher(t *testing.T) {
	g := NewGomegaWithT(t)

	configWatcher, err := NewConfigWatcher("192.0.0.1", 8000, "istio.cf.dev01.aws.istio.sapcloud.io", 9000)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(configWatcher).NotTo(BeNil())
}

func TestGetSnapshot(t *testing.T) {
	g := NewGomegaWithT(t)

	configWatcher, err := NewConfigWatcher("192.0.0.1", 8000, "istio.cf.dev01.aws.istio.sapcloud.io", 9000)
	snapshot, err := configWatcher.getSnapshot()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(snapshot).NotTo(BeNil())
	serviceEntries := snapshot.Resources(model.ServiceEntry.Collection)
	g.Expect(serviceEntries).To(HaveLen(1))
	g.Expect(serviceEntries[0].Metadata.Name).To(Equal("pinger-service-entry"))
	serviceEntry := &networking.ServiceEntry{}
	unWrapResource(serviceEntries[0], serviceEntry)
	g.Expect(serviceEntry.Hosts).To(HaveLen(1))
	g.Expect(serviceEntry.Hosts[0]).To(Equal("pinger.service-fabrik"))

	virtualServices := snapshot.Resources(model.VirtualService.Collection)
	g.Expect(virtualServices).To(HaveLen(1))
	g.Expect(virtualServices[0].Metadata.Name).To(Equal("pinger-virtual-service"))
	virtualService := &networking.VirtualService{}
	unWrapResource(virtualServices[0], virtualService)
	g.Expect(virtualService.Hosts).To(HaveLen(1))
	g.Expect(virtualService.Hosts[0]).To(Equal("pinger.istio.cf.dev01.aws.istio.sapcloud.io"))
	g.Expect(virtualService.Tcp[0].Route[0].Destination.Host).To(Equal("pinger.service-fabrik"))
	g.Expect(virtualService.Tcp[0].Route[0].Destination.Port.GetNumber()).To(Equal(uint32(8000)))

	gateways := snapshot.Resources(model.Gateway.Collection)
	g.Expect(gateways).To(HaveLen(1))
	g.Expect(gateways[0].Metadata.Name).To(Equal("pinger-gateway"))
	gateway := &networking.Gateway{}
	unWrapResource(gateways[0], gateway)
	g.Expect(gateway.Servers[0].Hosts[0]).To(Equal("pinger.istio.cf.dev01.aws.istio.sapcloud.io"))
	g.Expect(gateway.Servers[0].Port.Number).To(Equal(uint32(9000)))

}

func unWrapResource(resource *mcp.Resource, message proto.Message) error {
	return types.UnmarshalAny(resource.Body, message)
}
