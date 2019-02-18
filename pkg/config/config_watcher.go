package config

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"io/ioutil"
	mcp "istio.io/api/mcp/v1alpha1"
	"istio.io/istio/galley/pkg/metadata"
	"istio.io/istio/pilot/pkg/config/kube/crd"
	"istio.io/istio/pkg/mcp/snapshot"
	"log"
	"os"
	"path/filepath"
	"time"
)

type configWatcher struct {
	*snapshot.Cache
	watcher     *fsnotify.Watcher
	doneChannel chan struct{}
}

func NewConfigWatcher(dirname string) (*configWatcher, error) {

	var version = 1
	result := &configWatcher{
		Cache:       snapshot.New(func(collection string, node *mcp.SinkNode) string {
			return "default"
		}),
		doneChannel: make(chan struct{}),
	}
	snapshot, err := readSnapshotFromDirectory(dirname, version)
	if err != nil {
		return nil, err
	}
	result.SetSnapshot("default", snapshot)
	result.watcher, err = fsnotify.NewWatcher()
	result.watcher.Add(dirname)
	go func() {
		for {
			select {
			// watch for events
			case event, more := <-result.watcher.Events:
				if event.Op == fsnotify.Create || event.Op == fsnotify.Remove || event.Op == fsnotify.Write {
					if !more {
						break
					}
					version++
					snapshot, err := readSnapshotFromDirectory(dirname, version)
					if err != nil {
						log.Printf("Can't read configuration from directory %s: %s", dirname, err.Error())
					} else {
						result.SetSnapshot("default", snapshot)
					}
				}
				break
			case <-result.doneChannel:
				return
			}
		}
	}()

	return result, nil
}

func (c *configWatcher) Stop() {
	close(c.doneChannel)
	c.watcher.Close()
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

func readSnapshotFromDirectory(dirname string, version int) (snapshot.Snapshot, error) {
	configs := make(map[string][]namedSpec)
	err := filepath.Walk(dirname, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return readConfigMapFromFile(path, configs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return configMapToSnapshot(configs, version)
}

func readConfigMapFromFile(fileName string, configs map[string][]namedSpec) error {
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("unable to read file %s: %v",fileName,err)
	}

	istioConfigs, _, err := crd.ParseInputs(string(content))
	if err != nil {
		return fmt.Errorf("unable to parse content of file %s: %v",fileName,err)
	}

	for _, config := range istioConfigs {
		configs[config.Type] = append(configs[config.Type], namedSpec{config.Name, config.Spec})
	}
	return nil
}

func configMapToSnapshot(configs map[string][]namedSpec, version int) (snapshot.Snapshot, error) {
	resourceWrapper := resourceWrapper{}

	stringVersion := fmt.Sprintf("%d.0", version)
	snapshot := snapshot.NewInMemoryBuilder()
	for ctype, config := range configs {

		switch ctype {
		case "gateway":
			snapshot.Set(metadata.IstioNetworkingV1alpha3Gateways.Collection.String(), stringVersion, resourceWrapper.wrapMultiple(config))
		case "virtual-service":
			snapshot.Set(metadata.IstioNetworkingV1alpha3Virtualservices.Collection.String(), stringVersion, resourceWrapper.wrapMultiple(config))
		case "service-entry":
			snapshot.Set(metadata.IstioNetworkingV1alpha3Serviceentries.Collection.String(), stringVersion, resourceWrapper.wrapMultiple(config))
		default:
			return nil, fmt.Errorf("proto format error: config type %s unknown", ctype)
		}

	}

	return snapshot.Build(), nil

}
