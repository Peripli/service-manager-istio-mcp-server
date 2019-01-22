package config

import (
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	mcp "istio.io/api/mcp/v1alpha1"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/mcp/snapshot"
	"time"
)

type configWatcher struct {
	*snapshot.Cache
	pingerHost       string
	pingerPort       int
	systemDomain     string
	loadBalancerPort int
}

func NewConfigWatcher(pingerHost string, pingerPort int, systemDomain string, loadBalancerPort int) (*configWatcher, error) {

	result := &configWatcher{
		Cache:            snapshot.New(snapshot.DefaultGroupIndex),
		pingerHost:       pingerHost,
		pingerPort:       pingerPort,
		systemDomain:     systemDomain,
		loadBalancerPort: loadBalancerPort,
	}
	snapshot, err := result.getSnapshot()
	if err != nil {
		return nil, err
	}
	result.SetSnapshot("test", snapshot)
	return result, nil
}

func (c *configWatcher) getSnapshot() (snapshot.Snapshot, error) {
	serviceName := "pinger"
	hostName := "pinger." + c.systemDomain
	resourceWrapper := resourceWrapper{}
	builder := snapshot.NewInMemoryBuilder()
	builder.Set(model.ServiceEntry.Collection, "1.0",
		resourceWrapper.wrap(createRawServiceEntryForExternalService(c.pingerHost, uint32(c.pingerPort), serviceName), serviceName+"-service-entry"),
	)
	builder.Set(model.VirtualService.Collection, "1.0",
		resourceWrapper.wrap(createRawIngressVirtualServiceForExternalService(hostName, uint32(c.pingerPort), serviceName), serviceName+"-virtual-service"),
	)
	builder.Set(model.Gateway.Collection, "1.0",
		resourceWrapper.wrap(createRawIngressGatewayForExternalService(hostName, uint32(c.loadBalancerPort), serviceName), serviceName+"-gateway"),
	)
	return builder.Build(), resourceWrapper.err
}

type resourceWrapper struct {
	createTime *types.Timestamp
	err        error
}

func (r *resourceWrapper) wrap(message proto.Message, name string) []*mcp.Resource {
	if r.err != nil {
		return nil
	}
	if r.createTime == nil {
		r.createTime, r.err = types.TimestampProto(time.Now())
		if r.err != nil {
			return nil
		}

	}
	var body *types.Any
	body, r.err = types.MarshalAny(message)
	if r.err != nil {
		return nil
	}
	return []*mcp.Resource{{
		Metadata: &mcp.Metadata{
			Name:       name,
			CreateTime: r.createTime,
		},
		Body: body,
	}}

}
