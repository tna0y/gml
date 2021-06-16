package main

import (
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/inhies/go-bytesize"
)

func getMemoryUsage(pidTarget uint32, devices []nvml.Device) (usage uint64, err error) {
	for _, device := range devices {
		pids, ret := nvml.DeviceGetComputeRunningProcesses(device)
		if ret != nvml.SUCCESS {
			err = fmt.Errorf("failed to get NVML compute running processes: %v", nvml.ErrorString(ret))
			return
		}
		for _, pid := range pids {
			if pid.Pid == pidTarget {
				usage += pid.UsedGpuMemory
			}
		}
		pids, ret = nvml.DeviceGetGraphicsRunningProcesses(device)
		if ret != nvml.SUCCESS {
			err = fmt.Errorf("failed to get NVML graphics running processes: %v", nvml.ErrorString(ret))
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

func monitorChild(errChan chan<- error, pid int, limit uint64, devices []nvml.Device) {
	ticker := time.Tick(time.Millisecond * 100)
	for {
		select {
		case <-ticker:
			usage, err := getMemoryUsage(uint32(pid), devices)
			if err != nil {
				errChan <- err
				return
			}
			if usage > limit {
				errChan <- fmt.Errorf("GPU memory usage %s exceeded limit %s",
					bytesize.New(float64(usage)).String(),
					bytesize.New(float64(limit)).String(),
				)
				return
			}
		}
	}
}

func getDevices() (devices []nvml.Device, err error) {
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		err = fmt.Errorf("unable to get device count: %v", nvml.ErrorString(ret))
		return
	}

	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			err = fmt.Errorf("unable to get device at index %d: %v", i, nvml.ErrorString(ret))
			return
		}
		devices = append(devices, device)
	}
	return
}

func startChild(args []string) (pid int, err error) {
	return syscall.ForkExec(args[0], args, nil)
}

func waitForChild(errChan <-chan error, pid int, signum syscall.Signal) (exitCode int, err error) {

	done := make(chan int)
	go func() {
		var waitStatus syscall.WaitStatus
		var rusage syscall.Rusage
		_, _ = syscall.Wait4(pid, &waitStatus, 0, &rusage)
		done <- waitStatus.ExitStatus()
	}()

	select {
	case exitCode = <-done:
		return
	case err := <-errChan:
		_ = syscall.Kill(pid, signum)
		return <-done, err
	}
}

func main() {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		log.Fatalf("failed to initialize NVML: %v", nvml.ErrorString(ret))
		return
	}

	defer func() {
		ret := nvml.Shutdown()
		if ret != nvml.SUCCESS {
			log.Fatalf("failed to shutdown NVML: %v", nvml.ErrorString(ret))
			return
		}
	}()

	devices, err := getDevices()
	if err != nil {
		log.Fatalf("failed to get GPU list: %v", err)
	}

	pid, err := startChild([]string{"ls", "-l", "/"})
	if err != nil {
		log.Fatalf("failed to start process: %v", err)
	}

	signum := syscall.SIGKILL
	limit := uint64(1024 * 1024)

	monitorChan := make(chan error)
	go monitorChild(monitorChan, pid, limit, devices)
	exitCode, err := waitForChild(monitorChan, pid, signum)
	if err != nil {
		log.Println(err.Error())
	}
	os.Exit(exitCode)
}
