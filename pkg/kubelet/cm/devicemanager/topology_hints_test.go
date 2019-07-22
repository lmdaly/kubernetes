/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package devicemanager

import (
	"reflect"
	"sort"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager"
	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager/socketmask"
)

func makeDevice(id string, numa int) pluginapi.Device {
	return pluginapi.Device{
		ID:       id,
		Topology: &pluginapi.TopologyInfo{Node: &pluginapi.NUMANode{int64(numa)}},
	}
}

func topologyHintLessThan(a topologymanager.TopologyHint, b topologymanager.TopologyHint) bool {
	if a.Preferred != b.Preferred {
		return a.Preferred == true
	}
	return a.SocketAffinity.IsNarrowerThan(b.SocketAffinity)
}

func makeSocketMask(sockets ...int) socketmask.SocketMask {
	mask, _ := socketmask.NewSocketMask(sockets...)
	return mask
}

func TestGetTopologyHints(t *testing.T) {
	tcases := []struct {
		description   string
		request       map[string]string
		devices       map[string][]pluginapi.Device
		expectedHints map[string][]topologymanager.TopologyHint
	}{
		{
			description: "Single Request, one device per socket",
			request: map[string]string{
				"testdevice": "1",
			},
			devices: map[string][]pluginapi.Device{
				"testdevice": []pluginapi.Device{
					makeDevice("Dev1", 0),
					makeDevice("Dev2", 1),
				},
			},
			expectedHints: map[string][]topologymanager.TopologyHint{
				"testdevice": []topologymanager.TopologyHint{
					{
						SocketAffinity: makeSocketMask(0),
						Preferred:      true,
					},
					{
						SocketAffinity: makeSocketMask(1),
						Preferred:      true,
					},
				},
			},
		},
		{
			description: "Request for 2, one device per socket",
			request: map[string]string{
				"testdevice": "2",
			},
			devices: map[string][]pluginapi.Device{
				"testdevice": []pluginapi.Device{
					makeDevice("Dev1", 0),
					makeDevice("Dev2", 1),
				},
			},
			expectedHints: map[string][]topologymanager.TopologyHint{
				"testdevice": []topologymanager.TopologyHint{
					{
						SocketAffinity: makeSocketMask(0, 1),
						Preferred:      true,
					},
				},
			},
		},
		{
			description: "Request for 2, 2 devices per socket",
			request: map[string]string{
				"testdevice": "2",
			},
			devices: map[string][]pluginapi.Device{
				"testdevice": []pluginapi.Device{
					makeDevice("Dev1", 0),
					makeDevice("Dev2", 1),
					makeDevice("Dev3", 0),
					makeDevice("Dev4", 1),
				},
			},
			expectedHints: map[string][]topologymanager.TopologyHint{
				"testdevice": []topologymanager.TopologyHint{
					{
						SocketAffinity: makeSocketMask(0),
						Preferred:      true,
					},
					{
						SocketAffinity: makeSocketMask(1),
						Preferred:      true,
					},
					{
						SocketAffinity: makeSocketMask(0, 1),
						Preferred:      false,
					},
				},
			},
		},
		{
			description: "2 device types, mixed configurationt",
			request: map[string]string{
				"testdevice1": "2",
				"testdevice2": "1",
			},
			devices: map[string][]pluginapi.Device{
				"testdevice1": []pluginapi.Device{
					makeDevice("Dev1", 0),
					makeDevice("Dev2", 1),
					makeDevice("Dev3", 0),
					makeDevice("Dev4", 1),
				},
				"testdevice2": []pluginapi.Device{
					makeDevice("Dev1", 0),
					makeDevice("Dev2", 1),
				},
			},
			expectedHints: map[string][]topologymanager.TopologyHint{
				"testdevice1": []topologymanager.TopologyHint{
					{
						SocketAffinity: makeSocketMask(0),
						Preferred:      true,
					},
					{
						SocketAffinity: makeSocketMask(1),
						Preferred:      true,
					},
					{
						SocketAffinity: makeSocketMask(0, 1),
						Preferred:      false,
					},
				},
				"testdevice2": []topologymanager.TopologyHint{
					{
						SocketAffinity: makeSocketMask(0),
						Preferred:      true,
					},
					{
						SocketAffinity: makeSocketMask(1),
						Preferred:      true,
					},
				},
			},
		},
	}

	for _, tc := range tcases {
		resourceList := v1.ResourceList{}
		for r := range tc.request {
			resourceList[v1.ResourceName(r)] = resource.MustParse(tc.request[r])
		}

		pod := makePod(resourceList)

		m := ManagerImpl{
			allDevices:       make(map[string]map[string]pluginapi.Device),
			healthyDevices:   make(map[string]sets.String),
			allocatedDevices: make(map[string]sets.String),
			podDevices:       make(podDevices),
			sourcesReady:     &sourcesReadyStub{},
			activePods:       func() []*v1.Pod { return []*v1.Pod{} },
		}

		for r := range tc.devices {
			m.allDevices[r] = make(map[string]pluginapi.Device)
			m.healthyDevices[r] = sets.NewString()

			for _, d := range tc.devices[r] {
				m.allDevices[r][d.ID] = d
				m.healthyDevices[r].Insert(d.ID)
			}
		}

		hints := m.GetTopologyHints(*pod, pod.Spec.Containers[0])

		for r := range tc.expectedHints {
			sort.SliceStable(hints[r], func(i, j int) bool {
				return topologyHintLessThan(hints[r][i], hints[r][j])
			})
			sort.SliceStable(tc.expectedHints[r], func(i, j int) bool {
				return topologyHintLessThan(tc.expectedHints[r][i], tc.expectedHints[r][j])
			})
			if !reflect.DeepEqual(hints[r], tc.expectedHints[r]) {
				t.Errorf("%v: Expected result to be %v, got %v", tc.description, tc.expectedHints[r], hints[r])
			}
		}
	}
}
