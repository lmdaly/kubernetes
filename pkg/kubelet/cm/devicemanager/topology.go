package devicemanager

import (
  	"reflect"
    	"k8s.io/klog"
    	"k8s.io/api/core/v1"
    	"k8s.io/apimachinery/pkg/util/sets"
    	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager"
	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager/socketmask"
    	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

type topology struct {
    	largestSocket int64
       	deviceHints []topologymanager.TopologyHint	
    	admit bool
    	allDevices map[string][]pluginapi.Device
}

func (m *ManagerImpl) GetTopologyHints(pod v1.Pod, container v1.Container) ([]topologymanager.TopologyHint, bool) {
        topo := &topology{
		largestSocket:   int64(-1),
            	admit:       true,
            	allDevices:     m.allDevices,
        }
    	klog.Infof("Devices in GetTopologyHints: %v", topo.allDevices)
        var finalCrossSocketMask socketmask.SocketMask
        count := false
        topo.largestSocket = topo.getLargestSocket()
        deviceTriggered := false
        klog.Infof("Largest Socket in Devices: %v", topo.largestSocket)
	for resourceObj, amountObj := range container.Resources.Requests {
        	resource := string(resourceObj)
            	amount := int64(amountObj.Value())
            	if m.isDevicePluginResource(resource){
                	deviceTriggered = true
                	klog.Infof("%v is a resource managed by device manager.", resource)
                    	klog.Infof("Health Devices: %v", m.healthyDevices[resource])
                	if _, ok := m.healthyDevices[resource]; !ok {
                    		klog.Infof("No healthy devices for resource %v", resource)
                    		continue
                	}
                    	available := m.getAvailableDevices(resource)
                	
                	if int64(available.Len()) < amount {
                		klog.Infof("requested number of devices unavailable for %s. Requested: %d, Available: %d", resource, amount, available.Len())
                        	continue
                	}
                	klog.Infof("[devicemanager] Available devices for resource %v: %v", resource, available)
                	deviceSocketAvail := topo.getDevicesPerSocket(resource, available)
               
                	var mask socketmask.SocketMask
                	var crossSocket socketmask.SocketMask
                	crossSocket = make([]int64, (topo.largestSocket+1))
			var overwriteDeviceHint []topologymanager.TopologyHint
                	for socket, amountAvail := range deviceSocketAvail {
                    		klog.Infof("Socket: %v, Avail: %v", socket, amountAvail)
                    		mask = nil
                    		if amountAvail >= amount {
                        		mask = topo.calculateDeviceMask(socket)
                        		klog.Infof("Mask: %v", mask)
                        		if !count {
                                        klog.Infof("Not Count. Device Mask: %v", topo.deviceHints)
						var deviceHintsTemp topologymanager.TopologyHint
						deviceHintsTemp.SocketMask = mask
                            			topo.deviceHints = append(topo.deviceHints, deviceHintsTemp)
                        		} else {
                                        klog.Infof("Count. Device Mask: %v", topo.deviceHints)
                                        overwriteDeviceHint = append(overwriteDeviceHint, checkIfMaskEqualsStoreMask(topo.deviceHints, mask)...)
                                        klog.Infof("OverwriteDeviceHint: %v", overwriteDeviceHint)                                      
                        		}                           
                    		}	 
                    		//crossSocket can be duplicate of mask need to remove if so
                    		crossSocket[socket] = 1                 
                	}                                      
                	if !count {
                            	finalCrossSocketMask = crossSocket
                	} else {
                            	topo.deviceHints = overwriteDeviceHint
                            	klog.Infof("DeviceMask: %v", topo.deviceHints)    
                            	if !reflect.DeepEqual(finalCrossSocketMask, crossSocket) {
                                	finalCrossSocketMask = topo.calculateAllDeviceMask(finalCrossSocketMask, crossSocket)
                            	}                           
                	}                    
                	klog.Infof("deviceHints: %v", topo.deviceHints)
                    	klog.Infof("finalCrossSocketMask: %v", finalCrossSocketMask)
                   
                	count = true
        	}
	}
	var finalTopologyHint topologymanager.TopologyHint
    	finalTopologyHint.SocketMask = finalCrossSocketMask
    	if deviceTriggered {
            	topo.deviceHints = append(topo.deviceHints, finalTopologyHint)
            	topo.admit = calculateIfDeviceHasSocketAffinity(topo.deviceHints)
    	}
    	klog.Infof("DeviceMask %v: Device Affinity: %v", topo.deviceHints, topo.admit)

	return topo.deviceHints, topo.admit
}

func (m *ManagerImpl) getAvailableDevices(resource string) sets.String{
    // Gets Devices in use.
    m.updateAllocatedDevices(m.activePods())
    devicesInUse := m.allocatedDevices[resource]
    klog.Infof("Devices in use:%v", devicesInUse)

    // Gets a list of available devices.
    available := m.healthyDevices[resource].Difference(devicesInUse)
    return available
}

func (t *topology) getLargestSocket() int64 {
    largestSocket := int64(-1)
    for _, list := range t.allDevices {
            for _, device := range list {
                if device.Topology.Socket > largestSocket {
                    largestSocket = device.Topology.Socket
                }
            }
        }
    return largestSocket
}

func (t *topology) getDevicesPerSocket(resource string, available sets.String) map[int64]int64 {
    deviceSocketAvail := make(map[int64]int64)             
    for availID := range available {
        for _, device := range t.allDevices[resource] {
                klog.Infof("[device-manager] AvailID: %v DeviceID: %v", availID, device)
                if availID == device.ID {
                        socket := device.Topology.Socket
                    deviceSocketAvail[socket] += 1
                }                            
            }
    }
    return deviceSocketAvail
}

func (t *topology) calculateDeviceMask(socket int64) socketmask.SocketMask {
    var mask socketmask.SocketMask
    for i := int64(0); i < t.largestSocket+1; i++ {
        if i == socket {
            mask = append(mask, 1) 
        } else {
            mask = append(mask, 0)
        }
    }
    klog.Infof("Mask: %v", mask)
    return mask
}

func checkIfMaskEqualsStoreMask(existingDeviceHint []topologymanager.TopologyHint, newMask socketmask.SocketMask) []topologymanager.TopologyHint{
    var newDeviceHint []topologymanager.TopologyHint
    for _, storedHint := range existingDeviceHint {
        klog.Infof("For. StoredHint: %v", storedHint)
        if reflect.DeepEqual(storedHint.SocketMask, newMask) {
                klog.Infof("DeepEqual.")
                newDeviceHint = append(newDeviceHint, storedHint)
        }
    }
    return newDeviceHint
}

func (t *topology)calculateAllDeviceMask(finalSocketMask, crossSocket socketmask.SocketMask) socketmask.SocketMask{
    var tempSocketMask socketmask.SocketMask
    tempSocketMask = make([]int64, t.largestSocket+1)
    for i, bit := range finalSocketMask {
        klog.Infof("i %v for cross Socket, bit %v crossSocket[i] %v or result %v", bit, crossSocket[i], bit | crossSocket[i])
        tempSocketMask[i] = bit | crossSocket[i]
    }
    klog.Infof("TempSocketMask: %v", tempSocketMask)
    finalSocketMask = tempSocketMask 
    return finalSocketMask
}

func calculateIfDeviceHasSocketAffinity(deviceHints []topologymanager.TopologyHint) bool {
    admit := false
    for _, outerMask := range deviceHints {
        for _, innerMask := range outerMask.SocketMask {
            if innerMask == 0 {
                admit = true
                break
            }
        }
    }
    return admit
}
