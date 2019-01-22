package config

import (
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"io/ioutil"
	mcp "istio.io/api/mcp/v1alpha1"
	"istio.io/istio/galley/pkg/metadata"
	"istio.io/istio/pilot/pkg/config/kube/crd"
	"istio.io/istio/pkg/mcp/snapshot"
	"istio.io/istio/pkg/mcp/source"
	"time"
)

type configWatcher struct {
	*snapshot.Cache
	filename         string
	pingerHost       string
	pingerPort       int
	systemDomain     string
	loadBalancerPort int
}

func NewConfigWatcherInMem(pingerHost string, pingerPort int, systemDomain string, loadBalancerPort int) (*configWatcher, error) {

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
	result.SetSnapshot("default", snapshot)
	return result, nil
}

func NewConfigWatcher(filename string) (*configWatcher, error) {

	result := &configWatcher{
		Cache:    snapshot.New(snapshot.DefaultGroupIndex),
		filename: filename,
	}
	snapshot, err := result.readSnapshotFromFile()
	if err != nil {
		return nil, err
	}
	result.SetSnapshot("default", snapshot)
	return result, nil
}

func Collections() []source.CollectionOptions {
	return source.CollectionOptionsFromSlice([]string{
		metadata.Gateway.TypeURL.String(),
		metadata.VirtualService.TypeURL.String(),
		metadata.DestinationRule.TypeURL.String(),
		metadata.ServiceEntry.TypeURL.String(),
		metadata.EnvoyFilter.TypeURL.String(),
		metadata.HTTPAPISpec.TypeURL.String(),
		metadata.HTTPAPISpecBinding.TypeURL.String(),
		metadata.QuotaSpec.TypeURL.String(),
		metadata.QuotaSpecBinding.TypeURL.String(),
		metadata.Policy.TypeURL.String(),
		metadata.MeshPolicy.TypeURL.String(),
		metadata.ServiceRole.TypeURL.String(),
		metadata.ServiceRoleBinding.TypeURL.String(),
		metadata.RbacConfig.TypeURL.String(),
	})
}

func (c *configWatcher) getSnapshot() (snapshot.Snapshot, error) {
	serviceName := "pinger"
	hostName := "pinger." + c.systemDomain
	resourceWrapper := resourceWrapper{}
	builder := snapshot.NewInMemoryBuilder()
	builder.Set(metadata.ServiceEntry.TypeURL.String(), "1.0",
		[]*mcp.Resource{resourceWrapper.wrap(createRawServiceEntryForExternalService(c.pingerHost, uint32(c.pingerPort), serviceName), serviceName+"-service-entry")},
	)
	builder.Set(metadata.VirtualService.TypeURL.String(), "1.0",
		[]*mcp.Resource{resourceWrapper.wrap(createRawIngressVirtualServiceForExternalService(hostName, uint32(c.pingerPort), serviceName), serviceName+"-virtual-service")},
	)
	builder.Set(metadata.Gateway.TypeURL.String(), "1.0",
		[]*mcp.Resource{resourceWrapper.wrap(createRawIngressGatewayForExternalService(hostName, uint32(c.loadBalancerPort), serviceName), serviceName+"-gateway")},
	)
	return builder.Build(), resourceWrapper.err
}

type resourceWrapper struct {
	createTime *types.Timestamp
	err        error
}

type namedSpec struct {
	name string
	spec proto.Message
}

func (r *resourceWrapper) wrapMultiple(specs []namedSpec) []*mcp.Resource {
	resources := make([]*mcp.Resource, len(specs))
	for i, spec := range specs {
		resources[i] = r.wrap(spec.spec, spec.name)
	}
	return resources
}

func (r *resourceWrapper) wrap(message proto.Message, name string) *mcp.Resource {
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
	return &mcp.Resource{
		Metadata: &mcp.Metadata{
			Name:       name,
			CreateTime: r.createTime,
		},
		Body: body,
	}

}

func (c *configWatcher) readSnapshotFromFile() (snapshot.Snapshot, error) {
	content, err := ioutil.ReadFile(c.filename)
	if err != nil {
		return nil, err
	}

	istioConfigs, _, err := crd.ParseInputs(string(content))
	if err != nil {
		return nil, err
	}

	snapshot := snapshot.NewInMemoryBuilder()
	configs := make(map[string][]namedSpec)
	for _, config := range istioConfigs {
		configs[config.Type] = append(configs[config.Type], namedSpec{config.Name, config.Spec})
	}

	resourceWrapper := resourceWrapper{}

	for ctype, config := range configs {

		switch ctype {
		case "gateway":
			snapshot.Set(metadata.Gateway.TypeURL.String(), "1.0", resourceWrapper.wrapMultiple(config))
		case "virtual-service":
			snapshot.Set(metadata.VirtualService.TypeURL.String(), "1.0", resourceWrapper.wrapMultiple(config))
		case "service-entry":
			snapshot.Set(metadata.ServiceEntry.TypeURL.String(), "1.0", resourceWrapper.wrapMultiple(config))
		default:
			return nil, fmt.Errorf("Proto format error: config type %s unknown", ctype)
		}

	}

	return snapshot.Build(), nil
}
