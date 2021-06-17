package gml

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/inhies/go-bytesize"
	"golang.org/x/sync/errgroup"
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

func monitorChild(ctx context.Context, errChan chan<- error, pid int, limit uint64, devices []nvml.Device) {
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
		case <- ctx.Done():
			return
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

	path, err := exec.LookPath(args[0])
	if err != nil {
		err = fmt.Errorf("failed to lookup path for \"%s\": %v", args[0], err)
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		return 0, nil
	}

	procAttr := syscall.ProcAttr{
		Dir:   wd,
		Env:   os.Environ(),
		Files: []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
	}

	return syscall.ForkExec(path, args, &procAttr)
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


func passthroughSignals(ctx context.Context, pid int) {
	signalChan := make(chan os.Signal, 0x100)
	for i := 1; i < 65; i++ {
		if syscall.Signal(i) == syscall.SIGCHLD { // we need it for waiting
			continue
		}
		signal.Notify(signalChan, syscall.Signal(i))
	}
	for {
		select {
		case sig := <- signalChan:
			_ = syscall.Kill(pid, sig.(syscall.Signal))
		case <- ctx.Done():
			return
		}
	}
}

func Run(command []string, limit uint64, sig syscall.Signal) (code int, err error) {

	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		err = fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
		return
	}

	defer func() {
		ret := nvml.Shutdown()
		if ret != nvml.SUCCESS {
			if err == nil {
				err = fmt.Errorf("failed to shutdown NVML: %v", nvml.ErrorString(ret))
			}
			return
		}
	}()

	devices, err := getDevices()
	if err != nil {
		err = fmt.Errorf("failed to get GPU list: %v", err)
	}

	pid, err := startChild(command)
	if err != nil {
		err = fmt.Errorf("failed to start process: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	wg, wgCtx := errgroup.WithContext(ctx)

	wg.Go(func() error {
		passthroughSignals(wgCtx, pid)
		return nil
	})

	monitorChan := make(chan error)
	wg.Go(func() error {
		monitorChild(wgCtx, monitorChan, pid, uint64(limit), devices)
		return nil
	})

	exitCode, err := waitForChild(monitorChan, pid, sig)

	cancel()
	_ = wg.Wait()

	return exitCode, err
}