/*Copyright 2015 The Kubernetes Authors.
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
	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager/socketmask"
 	"k8s.io/klog"	
 	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/kubelet/lifecycle"
)
 
type Manager interface {
 	lifecycle.PodAdmitHandler
 	AddHintProvider(HintProvider)
	AddPod(pod *v1.Pod, containerID string) error 
 	RemovePod(containerID string) error 
 	Store
 }

type TopologyHints struct {
	SocketAffinity []socketmask.SocketMask
	Affinity bool
}
 
type manager struct {
 	//The list of components registered with the Manager
 	hintProviders []HintProvider
 	//List of Containers and their Topology Allocations
 	podTopologyHints map[string]containers
	podMap map[string]string	
    	//Topology Manager Policy
    	policy Policy
}
 
//Interface to be implemented by Topology Allocators 
type HintProvider interface {
    	GetTopologyHints(pod v1.Pod, container v1.Container) TopologyHints
}
 
type Store interface {
	GetAffinity(podUID string, containerName string) TopologyHints
}

 
type containers map[string]TopologyHints
var _ Manager = &manager{}
type policyName string
func NewManager(topologyPolicyName string) Manager {
 	klog.Infof("[topologymanager] Creating topology manager with %s policy", topologyPolicyName)
    	var policy Policy 
    
    	switch policyName(topologyPolicyName) {
    
    	case PolicyPreferred:
        	policy = NewPreferredPolicy()

     	case PolicyStrict:
        	policy = NewStrictPolicy()    
    
    	default:
        	klog.Errorf("[topologymanager] Unknow policy %s, using default policy %s", topologyPolicyName, PolicyPreferred)
		policy = NewPreferredPolicy()
    	}    
    
 	var hp []HintProvider
 	pnh := make (map[string]containers)
	pm := make (map[string]string)
 	manager := &manager{
 		hintProviders: hp,
 		podTopologyHints: pnh,
		podMap: pm,
        	policy: policy,
 	}
 	
	return manager
}

func (m *manager) GetAffinity(podUID string, containerName string) TopologyHints {
 	return m.podTopologyHints[podUID][containerName]
}

func (m *manager) calculateTopologyAffinity(pod v1.Pod, container v1.Container) TopologyHints {
	socketMask := socketmask.NewSocketMask(nil)
	var maskHolder []string
	count := 0 
	affinity := true
        for _, hp := range m.hintProviders {
		topologyHints := hp.GetTopologyHints(pod, container)
		if topologyHints.Affinity && topologyHints.SocketAffinity  != nil {
			socketMask, maskHolder = socketMask.GetSocketMask(topologyHints.SocketAffinity, maskHolder, count)
			count++
		} else if topologyHints.Affinity && topologyHints.SocketAffinity  == nil {
			klog.Infof("[topologymanager] NO Topology Affinity.")
			return TopologyHints {
                   		SocketAffinity: []socketmask.SocketMask{socketMask},
                    		Affinity:       false,
                	}
		} else if  !topologyHints.Affinity && topologyHints.SocketAffinity  != nil {
			klog.Infof("[topologymanager] Cross Socket Topology Affinity")
			affinity = false
			socketMask, maskHolder = socketMask.GetSocketMask(topologyHints.SocketAffinity, maskHolder, count)
			count++
		}
	}
	return TopologyHints {
		SocketAffinity: []socketmask.SocketMask{socketMask},
		Affinity:	affinity,
	}      
}

func (m *manager) AddHintProvider(h HintProvider) {
 	m.hintProviders = append(m.hintProviders, h)
}

func (m *manager) AddPod(pod *v1.Pod, containerID string) error {
	m.podMap[containerID] = string(pod.UID)
	return nil
}

func (m *manager) RemovePod (containerID string) error {
 	podUIDString := m.podMap[containerID]
	delete(m.podTopologyHints, podUIDString)
	delete(m.podMap, containerID)
	klog.Infof("[topologymanager] RemovePod - Container ID: %v podTopologyHints: %v", containerID, m.podTopologyHints)
	return nil 
}

func (m *manager) Admit(attrs *lifecycle.PodAdmitAttributes) lifecycle.PodAdmitResult {
 	klog.Infof("[topologymanager] Topology Admit Handler")
 	pod := attrs.Pod
 	c := make (containers)
	klog.Infof("[topologymanager] Pod QoS Level: %v", pod.Status.QOSClass)
	
	qosClass := pod.Status.QOSClass
	
	if qosClass == "Guaranteed" {
		for _, container := range pod.Spec.Containers {
			result := m.calculateTopologyAffinity(*pod, container)
			admitPod := m.policy.CanAdmitPodResult(result)
            		if admitPod.Admit == false {
                		return admitPod
            		}
			c[container.Name] = result
		}
		m.podTopologyHints[string(pod.UID)] = c
		klog.Infof("[topologymanager] Topology Affinity for Pod: %v are %v", pod.UID, m.podTopologyHints[string(pod.UID)])
	
	} else {
		klog.Infof("[topologymanager] Topology Manager only affinitises Guaranteed pods.")
	}
	
	return lifecycle.PodAdmitResult{
		Admit:   true,
	}
}
