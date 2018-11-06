package socketmask

import (
	"strconv"
        "strings"
        "bytes"
        "math"
        "github.com/golang/glog"
)


type SocketMask []int64

func NewSocketMask(Mask []int64) SocketMask {
	sm := Mask
	return sm
}

func (sm SocketMask) BuildMaskHolder(mask [][]int64) []string {
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

func (sm SocketMask) GetTopologyAffinity(arrangedMask, maskHolder []string) []string {
        var topologyTemp []string
        for i:= 0; i < (len(maskHolder)); i++ {
                for j:= 0; j < (len(arrangedMask)); j++ {
                        tempStr := sm.AndOperation(maskHolder[i],arrangedMask[j])
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

func (sm SocketMask) ParseMask(mask []string) []int64 {
        var maskStr string
        min := strings.Count(mask[0], "1")
        var num, index int

        for i := 0; i < len(mask); i++ {
                num = strings.Count(mask[i], "1")
                if num < min {
                        min = num
                        index = i
                }
                maskStr = mask[index]
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

func (sm SocketMask) ArrangeMask(mask [][]int64) []string {
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

func (sm SocketMask) AndOperation(val1, val2 string) (string) {
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

