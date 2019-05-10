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

package topologymanager

import (
	"reflect"
	"testing"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager/socketmask"
	"k8s.io/kubernetes/pkg/kubelet/lifecycle"
)

func TestNewManager(t *testing.T) {
	tcases := []struct {
		name       string
		policyType string
	}{
		{
			name:       "Policy is set preferred",
			policyType: "preferred",
		},
		{
			name:       "Policy is set to strict",
			policyType: "strict",
		},
		{
			name:       "Policy is set to unknown",
			policyType: "unknown",
		},
	}

	for _, tc := range tcases {
		mngr := NewManager(tc.policyType)

		if _, ok := mngr.(Manager); !ok {
			t.Errorf("result is not Manager type")
		}
	}
}

type mockHintProvider struct {
	th	[]TopologyHint
	admit	bool
}

func (m *mockHintProvider) GetTopologyHints(pod v1.Pod, container v1.Container) ([]TopologyHint, bool) {
	return m.th, m.admit
}

func TestGetAffinity(t *testing.T) {
	tcases := []struct {
		name          string
		containerName string
		podUID        string
		expected      TopologyHint
	}{
		{
			name:          "case1",
			containerName: "nginx",
			podUID:        "0aafa4c4-38e8-11e9-bcb1-a4bf01040474",
			expected: 	TopologyHint{},
		},
	}
	for _, tc := range tcases {
		mngr := manager{}
		actual := mngr.GetAffinity(tc.podUID, tc.containerName)
		if !reflect.DeepEqual(actual, tc.expected) {
			t.Errorf("Expected Affinity in result to be %v, got %v", tc.expected, actual)
		}
	}
}

func TestCalculateTopologyAffinity(t *testing.T) {
	tcases := []struct {
		name     	string
		hp       	[]HintProvider
		expectedHint 	TopologyHint
		expectedAdmit	bool
	}{
		{
			name: 		"TopologyHint not set",
			hp:   		[]HintProvider{},
			expectedHint: 	TopologyHint{
					SocketMask:	nil,
			},
			expectedAdmit: 	true,
		},
		{
			name: "TopologyHint set with SocketMask as nil. Admit set as false",
			hp: []HintProvider{
				&mockHintProvider{
					th: 	[]TopologyHint{
							TopologyHint{
                                                        	SocketMask: nil,
                                                        },	
						},
					admit:	false,
				},
			},
			expectedHint:	TopologyHint{
				SocketMask: 	nil,
			},
			expectedAdmit:	false,
		},
		{
			name: "TopologyHint set with SocketMask as not nil. Admit set as false",
			hp: []HintProvider{
				&mockHintProvider{
					th:	[]TopologyHint{ 
							TopologyHint{
								SocketMask: socketmask.SocketMask{1, 0},
							},
							TopologyHint{
                                                                SocketMask: socketmask.SocketMask{0, 1},
                                                        }, 
							TopologyHint{
                                                                SocketMask: socketmask.SocketMask{1, 1},
                                                        }, 	
						},
					admit:	false,
				},
			},
			expectedHint: TopologyHint{
				SocketMask: socketmask.SocketMask{1, 0},
			},
			expectedAdmit:	false,
		},
		{
			name: "TopologyHint with SocketMask as not nil. Admit set as true",
			hp: []HintProvider{
				&mockHintProvider{
					th: 	[]TopologyHint{
							TopologyHint{
								SocketMask: socketmask.SocketMask{1, 0},
							},
							TopologyHint{
                                                                SocketMask: socketmask.SocketMask{0, 1},
                                                        },
							TopologyHint{
                                                                SocketMask: socketmask.SocketMask{1, 1},
                                                        },

						},
					admit: true,
				},
			},
			expectedHint:	TopologyHint{
				SocketMask: socketmask.SocketMask{1, 0},
			},
			expectedAdmit:	true,
		},
	}

	for _, tc := range tcases {
		mngr := manager{}
		mngr.hintProviders = tc.hp
		_, admit := mngr.calculateTopologyAffinity(v1.Pod{}, v1.Container{})
		if !reflect.DeepEqual(admit, tc.expectedAdmit) {
			t.Errorf("Expected Affinity in result to be %v, got %v", tc.expectedAdmit, admit)
		}
	}
}

func TestAddContainer(t *testing.T) {
	testCases := []struct {
		name        string
		containerID string
		podUID      types.UID
	}{
		{
			name:        "Case1",
			containerID: "nginx",
			podUID:      "0aafa4c4-38e8-11e9-bcb1-a4bf01040474",
		},
		{
			name:        "Case2",
			containerID: "Busy_Box",
			podUID:      "b3ee37fc-39a5-11e9-bcb1-a4bf01040474",
		},
	}
	mngr := manager{}
	mngr.podMap = make(map[string]string)
	for _, tc := range testCases {
		pod := v1.Pod{}
		pod.UID = tc.podUID
		err := mngr.AddContainer(&pod, tc.containerID)
		if err != nil {
			t.Errorf("Expected error to be nil but got: %v", err)
		}
		if val, ok := mngr.podMap[tc.containerID]; ok {
			if reflect.DeepEqual(val, pod.UID) {
				t.Errorf("Error occurred")
			}
		} else {
			t.Errorf("Error occurred, Pod not added to podMap")
		}
	}
}

func TestRemoveContainer(t *testing.T) {
	testCases := []struct {
		name        string
		containerID string
		podUID      types.UID
	}{
		{
			name:        "Case1",
			containerID: "nginx",
			podUID:      "0aafa4c4-38e8-11e9-bcb1-a4bf01040474",
		},
		{
			name:        "Case2",
			containerID: "Busy_Box",
			podUID:      "b3ee37fc-39a5-11e9-bcb1-a4bf01040474",
		},
	}
	var len1, len2 int
	mngr := manager{}
	mngr.podMap = make(map[string]string)
	for _, tc := range testCases {
		mngr.podMap[tc.containerID] = string(tc.podUID)
		len1 = len(mngr.podMap)
		err := mngr.RemoveContainer(tc.containerID)
		len2 = len(mngr.podMap)
		if err != nil {
			t.Errorf("Expected error to be nil but got: %v", err)
		}
		if len1-len2 != 1 {
			t.Errorf("Remove Pod resulted in error")
		}
	}

}
func TestAddHintProvider(t *testing.T) {
	var len1 int
	tcases := []struct {
		name string
		hp   []HintProvider
	}{
		{
			name: "Add HintProvider",
			hp: []HintProvider{
				&mockHintProvider{
					th:	[]TopologyHint{
							TopologyHint{
								SocketMask: nil,
							},
						},
					admit:	true,
				},
			},
		},
	}
	mngr := manager{}
	for _, tc := range tcases {
		mngr.hintProviders = []HintProvider{}
		len1 = len(mngr.hintProviders)
		mngr.AddHintProvider(tc.hp[0])
	}
	len2 := len(mngr.hintProviders)
	if len2-len1 != 1 {
		t.Errorf("error")
	}
}

func TestAdmit(t *testing.T) {
	tcases := []struct {
		name     string
		result   lifecycle.PodAdmitResult
		qosClass v1.PodQOSClass
		expected bool
	}{
		{
			name:     "QOSClass set as Gauranteed",
			result:   lifecycle.PodAdmitResult{},
			qosClass: "Guaranteed",
			expected: true,
		},
		{
			name:     "QOSClass set as Burstable",
			result:   lifecycle.PodAdmitResult{},
			qosClass: "Burstable",
			expected: true,
		},
		{
			name:     "QOSClass set as BestEffort",
			result:   lifecycle.PodAdmitResult{},
			qosClass: "BestEffort",
			expected: true,
		},
	}
	for _, tc := range tcases {
		man := manager{}
		man.podTopologyHints = make(map[string]containers)
		podAttr := lifecycle.PodAdmitAttributes{}
		pod := v1.Pod{}
		pod.Status.QOSClass = tc.qosClass
		podAttr.Pod = &pod
		//c := make(containers)
		actual := man.Admit(&podAttr)
		if reflect.DeepEqual(actual, tc.result) {
			t.Errorf("Error occured, expected Admit in result to be %v got %v", tc.result, actual.Admit)
		}
	}
}
