// Copyright 2018 Intel Corp. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"
	"time"
	"strconv"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	// Device plugin settings.
	pluginMountPath      = "/var/lib/kubelet/device-plugins"
	kubeletEndpoint      = "kubelet.sock"
	pluginEndpointPrefix = "numadp"
	resourceName         = "intel.com/numa_test_dp"
)

type testManager struct {
        socketFile      string
 	devices         map[string]*pluginapi.Device
 	deviceFiles     []string
        grpcServer      *grpc.Server
 }

func newTestManager() *testManager {
	return &testManager{
        	socketFile:   fmt.Sprintf("%s.sock", pluginEndpointPrefix),
 		devices:     make(map[string]*pluginapi.Device),
 		deviceFiles: []string{"/root/othertestdevice"},
 	}
}

func (test *testManager) discoverTestResources() error {
 	test.devices = make(map[string]*pluginapi.Device)
 	glog.Infof("Discovered Devices below:")
 		dev001 := pluginapi.Device{ID: "25102017", Health: pluginapi.Healthy, Topology:&pluginapi.TopologyInfo{Nodes: []*pluginapi.NUMANode{&pluginapi.NUMANode{ID: 0,},}}}
 	dev002 := pluginapi.Device{ID: "26102017", Health: pluginapi.Healthy, Topology:&pluginapi.TopologyInfo{Nodes: []*pluginapi.NUMANode{&pluginapi.NUMANode{ID: 1,},}}}
        dev003 := pluginapi.Device{ID: "27102017", Health: pluginapi.Healthy,Topology:&pluginapi.TopologyInfo{Nodes: []*pluginapi.NUMANode{&pluginapi.NUMANode{ID: 1,},}}}
 	test.devices["25102017"] = &dev001
 	test.devices["26102017"] = &dev002
        test.devices["27102017"] = &dev003
 	for k, dev := range test.devices {
 			fmt.Printf("device[Key =%v] Value= %v\n",k,dev)
       }
       return nil
}

func (test *testManager) GetDeviceState(DeviceName string) string {
	// TODO: Discover device health
	return pluginapi.Healthy
}

func (test *testManager) Start() error {
	glog.Infof("Discovering NUMA Test DP device[s]")
	if err := test.discoverTestResources(); err != nil {
		return err
	}
	pluginEndpoint := filepath.Join(pluginapi.DevicePluginPath, test.socketFile)
	glog.Infof("Starting NUMA Test Device Plugin server at: %s\n", pluginEndpoint)
	lis, err := net.Listen("unix", pluginEndpoint)
	if err != nil {
		glog.Errorf("Error. Starting SRIOV Network Device Plugin server failed: %v", err)
	}
	test.grpcServer = grpc.NewServer()

	// Register all services
	pluginapi.RegisterDevicePluginServer(test.grpcServer, test)
	//api.RegisterCniEndpointServer(test.grpcServer, test)

	go test.grpcServer.Serve(lis)

	// Wait for server to start by launching a blocking connection
	conn, err := grpc.Dial(pluginEndpoint, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		glog.Errorf("Error. Could not establish connection with gRPC server: %v", err)
		return err
	}
	glog.Infoln("NUMA Test Device Plugin server started serving")
	conn.Close()
	return nil
}

func (test *testManager) Stop() error {
	glog.Infof("Stopping NUMA test Device Plugin gRPC server..")
	return nil
}

// Removes existing socket if exists
// [adpoted from https://github.com/redhat-nfvpe/k8s-dummy-device-plugin/blob/master/dummy.go ]
func (test *testManager) cleanup() error {
	pluginEndpoint := filepath.Join(pluginapi.DevicePluginPath, test.socketFile)
	if err := os.Remove(pluginEndpoint); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// Register registers as a grpc client with the kubelet.
func Register(kubeletEndpoint, pluginEndpoint, resourceName string) error {
    	glog.Infof("NUMA Test DP Registering with Kubelet..")
	conn, err := grpc.Dial(kubeletEndpoint, grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	if err != nil {
		glog.Errorf("NUMA Test Device Plugin cannot connect to Kubelet service: %v", err)
		return err
	}
	defer conn.Close()
	client := pluginapi.NewRegistrationClient(conn)

	request := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     pluginEndpoint,
		ResourceName: resourceName,
	}

	if _, err = client.Register(context.Background(), request); err != nil {
		glog.Errorf("NUMA Test Device Plugin cannot register to Kubelet service: %v", err)
		return err
	}
	return nil
}

// Implements DevicePlugin service functions
func (test *testManager) ListAndWatch(empty *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	changed := true
	for {
		for id, dev := range test.devices {
			state := test.GetDeviceState(id)
			if dev.Health != state {
				changed = true
				dev.Health = state
				test.devices[id] = dev
			}
		}
		if changed {
			resp := new(pluginapi.ListAndWatchResponse)
			for _, dev := range test.devices {
				resp.Devices = append(resp.Devices, &pluginapi.Device{ID: dev.ID, Health: dev.Health, Topology: &pluginapi.TopologyInfo{Nodes: []*pluginapi.NUMANode{&pluginapi.NUMANode{ID: dev.Topology.Nodes[0].ID,}},}})
			}
			glog.Infof("ListAndWatch: send devices %v\n", resp)
			if err := stream.Send(resp); err != nil {
				glog.Errorf("Error. Cannot update device states: %v\n", err)
				//test.grpcServer.Stop()
				return err
			}
		}
		changed = false
		time.Sleep(5 * time.Second)
	}
	return nil
}

func (test *testManager) PreStartContainer(ctx context.Context, psRqt *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (test *testManager) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{
		PreStartRequired: false,
	}, nil
}

//Allocate passes the PCI Addr(s) as an env variable to the requesting container
func (test *testManager) Allocate(ctx context.Context, rqt *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	resp := new(pluginapi.AllocateResponse)
	ids := ""
	for _, container := range rqt.ContainerRequests {
		containerResp := new(pluginapi.ContainerAllocateResponse)
		for _, id := range container.DevicesIDs {
			glog.Infof("DeviceID in Allocate: %v", id)
			dev, ok := test.devices[id]
			if !ok {
				glog.Errorf("Error. Invalid allocation request with non-existing device %s", id)
				return nil, fmt.Errorf("Error. Invalid allocation request with non-existing device %s", id)
			}
			if dev.Health != pluginapi.Healthy {
				glog.Errorf("Error. Invalid allocation request with unhealthy device %s", id)
				return nil, fmt.Errorf("Error. Invalid allocation request with unhealthy device %s", id)
			}
			ids = ids + "{" + id + ", " + strconv.Itoa(int(dev.Topology.Nodes[0].ID)) + "}"
		}
		envmap := make(map[string]string)
		envmap[resourceName] = ids
		containerResp.Envs = envmap
		resp.ContainerResponses = append(resp.ContainerResponses, containerResp)
	}
	return resp, nil
} 



func main() {
	flag.Parse()
	glog.Infof("Starting NUMA Test Device Plugin...")
	test := newTestManager()
	if test == nil {
		glog.Errorf("Unable to get instance of a test-Manager")
		return
	}
	test.cleanup()

	// respond to syscalls for termination
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	// Start server
	if err := test.Start(); err != nil {
		glog.Errorf("NUMATestDp.Start() failed: %v", err)
		return
	}
    	glog.Infof("Started NUMA Test DP...")
	
	// Registers with Kubelet.
	err := Register(path.Join(pluginMountPath, kubeletEndpoint), test.socketFile, resourceName)
	if err != nil {
		glog.Fatal(err)
		return
	}
	glog.Infof("NUMA Test Device Plugin registered with the Kubelet")

	// Catch termination signals
	select {
	case sig := <-sigCh:
		glog.Infof("Received signal \"%v\", shutting down.", sig)
		test.Stop()
		return
	}
}
