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
 	"strconv"
 	"strings"
	"bytes"
	"math"
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

type SocketMask struct {
        Mask [][]int64
}

type TopologyHints struct {
	SocketAffinity SocketMask
	Affinity bool
}
 
type manager struct {
 	//The list of components registered with the Manager
 	hintProviders []HintProvider
 	//List of Containers and their Topology Allocations
 	podTopologyHints map[string]containers	
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
func NewManager() Manager {
 	glog.Infof("[topologymanager] Creating topology manager")
 	var hp []HintProvider
 	pnh := make (map[string]containers)
 	manager := &manager{
 		hintProviders: hp,
 		podTopologyHints: pnh,
 	}
 	
	return manager
}

func (m *manager) GetAffinity(podUID string, containerName string) TopologyHints {
 	return m.podTopologyHints[podUID][containerName]
}

func (m *manager) calculateTopologyAffinity(pod v1.Pod, container v1.Container) TopologyHints {
	socketMask := SocketMask{
		Mask:		nil,
	}
	
	podTopologyHints := TopologyHints {
		SocketAffinity:	socketMask,
		Affinity: 	true,
	}
		
	var maskHolder []string
	count := 0 
	var finalMaskValue []int64
        for _, hp := range m.hintProviders {
		for resource, amount := range container.Resources.Requests {
			glog.Infof("Container Resource Name in Topology Manager: %v, Amount: %v", resource, amount.Value())
			topologyHints := hp.GetTopologyHints(string(resource), int(amount.Value()))
			if topologyHints.Affinity && topologyHints.SocketAffinity.Mask != nil {
				if count == 0 {
					maskHolder = buildMaskHolder(topologyHints.SocketAffinity.Mask)
					count++
				}
				glog.Infof("[topologymanager] MaskHolder : %v", maskHolder)
				//Arrange int array into array of strings 
				glog.Infof("[topologymanager] %v is passed into arrange function",topologyHints.SocketAffinity.Mask)   
				arrangedMask := arrangeMask(topologyHints.SocketAffinity.Mask)						
				newMask := getTopologyAffinity(arrangedMask, maskHolder)
				glog.Infof("[topologymanager] New Mask after getTopologyAffinity (new mask) : %v ",newMask)
				finalMaskValue = parseMask(newMask)
				glog.Infof("[topologymanager] Mask []Int64 (finalMaskValue): %v", finalMaskValue)
				maskHolder = newMask
				glog.Infof("[topologymanager] New MaskHolder: %v", maskHolder) 
     
			} else if topologyHints.Affinity && topologyHints.SocketAffinity.Mask == nil {
				glog.Infof("[topologymanager] NO Topology Affinity.")
				return podTopologyHints
			
			}  
		}
	}
	var topologyMaskFull [][]int64
        topologyMaskFull = append(topologyMaskFull, finalMaskValue)
        podTopologyHints.SocketAffinity.Mask = topologyMaskFull
        return podTopologyHints      

}
func buildMaskHolder(mask [][]int64) []string {
	var maskHolder []string
	outerLen := len(mask)
        var innerLen int = 0 
      	for i := 0; i < outerLen; i++ {
        	if innerLen < len(mask[i]) {
          		innerLen = len(mask[i])
       		}
     	}
       	var buffer bytes.Buffer
   	var i, j int = 0, 0
       	for i = 0; i < outerLen; i++ {
    		for j = 0; j < innerLen; j++ {
            		buffer.WriteString("1")
     		}
         	maskHolder = append(maskHolder, buffer.String())
               	buffer.Reset()
     	}
	return maskHolder
}

func getTopologyAffinity(arrangedMask, maskHolder []string) []string {
	var topologyTemp []string
        for i:= 0; i < (len(maskHolder)); i++ {
        	for j:= 0; j < (len(arrangedMask)); j++ {
               		tempStr := andOperation(maskHolder[i],arrangedMask[j])
                      	if strings.Contains(tempStr, "1") {
                               	topologyTemp = append(topologyTemp, tempStr )
                      	}
               	}
     	}
        duplicates := map[string]bool{}
        for v:= range topologyTemp {
        	duplicates[topologyTemp[v]] = true
        }
       	// Place all keys from the map into a slice.
       	topologyResult := []string{}
      	for key, _ := range duplicates {
       		topologyResult = append(topologyResult, key)
     	}
	
	return topologyResult
}

func parseMask(mask []string) []int64 {
	var maskStr string
	var countSlice []int64
	for i := 0; i < len(mask); i++ {
		num := int64(strings.Count(mask[i], "1"))
		countSlice = append(countSlice, num)		
        }
	min := countSlice[0]
	for i := 0; i < len(countSlice); i++ {
		if (countSlice[i] < min) && (countSlice[i] >= 1) {
			min = countSlice[i]
		}			
	}
	for i := 0; i < len(countSlice); i++ {
		if min == countSlice[i]{
			maskStr = mask[i]
			break
		}	
	}

	var maskInt []int64
	for _, char := range maskStr {
		convertedStr, err := strconv.Atoi(string(char))
		if err != nil {
			glog.Errorf("Could not convert string to int. Err: %v", err)
			return maskInt
		}
		maskInt = append(maskInt, int64(convertedStr))  
	}
	glog.Infof("Mask Int in Parse Mask: %v", maskInt)
	return maskInt         
}

func arrangeMask(mask [][]int64) []string {
	var socketStr []string
	var bufferNew bytes.Buffer
	outerLen := len(mask)
	innerLen := len(mask[0])
	for i := 0; i < outerLen; i++ {
		for j := 0; j < innerLen; j++ {
			if mask[i][j] == 1 {
				bufferNew.WriteString("1")
			} else if mask[i][j] == 0 {
				bufferNew.WriteString("0")
			}
		}
		socketStr = append(socketStr, bufferNew.String())
		bufferNew.Reset()
	}
	return socketStr
}

func andOperation(val1, val2 string) (string) {
	l1, l2 := len(val1), len(val2)
	//compare lengths of strings - pad shortest with trailing zeros
	if l1 != l2 {
		// Get the bit difference
		var num int
		diff := math.Abs(float64(l1) - float64(l2))
		num = int(diff)
		if l1 < l2 {
			val1 = val1 + strings.Repeat("0", num)
		} else {
			val2 = val2 + strings.Repeat("0", num)
		}
	}
	length := len(val1)
	byteArr := make([]byte, length)
	for i := 0; i < length ; i++ {
		byteArr[i] = (val1[i] & val2[i])
    	}
	var finalStr string
	finalStr = string(byteArr[:])	
	
	return finalStr
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
			if result.Affinity == true && result.SocketAffinity.Mask == nil {
				return lifecycle.PodAdmitResult{
					Admit:   false,
					Reason:	 "Topology Affinity Error",
					Message: "Resources cannot be allocated with Topology Locality",
					
				}
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
