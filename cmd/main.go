package main

import (
	"fmt"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/inhies/go-bytesize"
	"log"
	"time"
)

func getMemoryUsage(pidTarget uint32, devices []nvml.Device) (usage uint64, ret nvml.Return) {
	var pids []nvml.ProcessInfo
	for _, device := range devices {
		pids, ret = nvml.DeviceGetComputeRunningProcesses(device)
		if ret != nvml.SUCCESS {
			return
		}
		for _, pid := range pids {
			if pid.Pid == pidTarget {
				usage += pid.UsedGpuMemory
			}
		}
		pids, ret = nvml.DeviceGetGraphicsRunningProcesses(device)
		if ret != nvml.SUCCESS {
			return
		}
		for _, pid := range pids {
			if pid.Pid == pidTarget {
				usage += pid.UsedGpuMemory
			}
		}
	}
	return
}

func main() {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		log.Fatalf("Unable to initialize NVML: %v", nvml.ErrorString(ret))
	}
	defer func() {
		ret := nvml.Shutdown()
		if ret != nvml.SUCCESS {
			log.Fatalf("Unable to shutdown NVML: %v", nvml.ErrorString(ret))
		}
	}()

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		log.Fatalf("Unable to get device count: %v", nvml.ErrorString(ret))
	}
	fmt.Println("device count", count)

	var devices []nvml.Device
	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			log.Fatalf("Unable to get device at index %d: %v", i, nvml.ErrorString(ret))
		}
		devices = append(devices, device)
	}

	for {
		usage, ret := getMemoryUsage(19505, devices)
		if ret != nvml.SUCCESS {
			log.Fatalf("Unable to get memory reading: %v", nvml.ErrorString(ret))
		}
		log.Println(float64(usage) / 1024 / 1024 / 1024)
		time.Sleep(time.Second)
	}
}
