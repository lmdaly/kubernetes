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
 	"github.com/golang/glog"	
 	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/kubelet/lifecycle"
)
 
type Manager interface {
 	lifecycle.PodAdmitHandler
 	AddHintProvider(HintProvider)
 	RemovePod(podName string)
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
    	//Topology Manager Policy
    	policy Policy
}
 
//Interface to be implemented by Topology Allocators 
type HintProvider interface {
    	GetTopologyHints(resource string, amount int) TopologyHints
}
 
type Store interface {
	GetAffinity(podUID string, containerName string) TopologyHints
}

 
type containers map[string]TopologyHints
var _ Manager = &manager{}
type policyName string
func NewManager(topologyPolicyName string) Manager {
 	glog.Infof("[topologymanager] Creating topology manager with %s policy", topologyPolicyName)
    	var policy Policy 
    
    	switch policyName(topologyPolicyName) {
    
    	case PolicyPreferred:
        	policy = NewPreferredPolicy()

     	case PolicyStrict:
        	policy = NewStrictPolicy()    
    
    	default:
        	glog.Errorf("[topologymanager] Unknow policy %s, using default policy %s", topologyPolicyName, PolicyPreferred)
		policy = NewPreferredPolicy()
    	}    
    
 	var hp []HintProvider
 	pnh := make (map[string]containers)
 	manager := &manager{
 		hintProviders: hp,
 		podTopologyHints: pnh,
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
        for _, hp := range m.hintProviders {
		for resource, amount := range container.Resources.Requests {
			glog.Infof("Container Resource Name in Topology Manager: %v, Amount: %v", resource, amount.Value())
			topologyHints := hp.GetTopologyHints(string(resource), int(amount.Value()))
			if topologyHints.Affinity && topologyHints.SocketAffinity  != nil {
				socketMask, maskHolder = socketMask.GetSocketMask(topologyHints.SocketAffinity, maskHolder, count)
				count++
			} else if topologyHints.Affinity && topologyHints.SocketAffinity  == nil {
				glog.Infof("[topologymanager] NO Topology Affinity.")
				return TopologyHints {
			                SocketAffinity: []socketmask.SocketMask{socketMask},
                			Affinity:       true,
        			}
			}
		}
	}
	return TopologyHints {
		SocketAffinity: []socketmask.SocketMask{socketMask},
		Affinity:	true,
	}      
}

func (m *manager) AddHintProvider(h HintProvider) {
 	m.hintProviders = append(m.hintProviders, h)
}

func (m *manager) RemovePod (podName string) {
 	glog.Infof("Remove pod func")
}

func (m *manager) Admit(attrs *lifecycle.PodAdmitAttributes) lifecycle.PodAdmitResult {
 	glog.Infof("[topologymanager] Topology Admit Handler")
 	pod := attrs.Pod
 	c := make (containers)
 	
	glog.Infof("[topologymanager] Pod QoS Level: %v", pod.Status.QOSClass)
	
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
		glog.Infof("[topologymanager] Topology Affinity for Pod: %v are %v", pod.UID, m.podTopologyHints[string(pod.UID)])
	
	} else {
		glog.Infof("[topologymanager] Topology Manager only affinitises Guaranteed pods.")
	}
	
	return lifecycle.PodAdmitResult{
		Admit:   true,
	}
}
