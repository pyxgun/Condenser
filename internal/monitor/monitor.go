package monitor

import (
	"condenser/internal/store/csm"
	"condenser/internal/utils"
	"log"
	"syscall"
	"time"
)

func NewContainerMonitor() *ContainerMonitor {
	return &ContainerMonitor{
		csmHandler: csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
	}
}

type ContainerMonitor struct {
	csmHandler csm.CsmHandler
}

func (m *ContainerMonitor) Start() error {
	for {
		time.Sleep(100 * time.Millisecond)

		// get container list
		containerList, _ := m.csmHandler.GetContainerList()

		for _, container := range containerList {
			// status check
			// monitoring target: created, running
			if container.State != "running" && container.State != "created" {
				continue
			}
			// send keep alive
			procExist, _ := m.pidAlive(container.Pid)
			// if process is not exist, change state to stopped
			if !procExist {
				log.Printf("[*] Container: %s down detected.", container.ContainerId)
				if err := m.csmHandler.UpdateContainer(
					container.ContainerId,
					"stopped",
					0,
				); err != nil {
					continue
				}
			}
		}
	}
}

func (m *ContainerMonitor) pidAlive(pid int) (bool, error) {
	if pid <= 0 {
		// process not exist
		return false, nil
	}

	// send 0 signal to process
	err := syscall.Kill(pid, 0)
	switch err {
	case nil:
		// process exist
		return true, nil
	case syscall.ESRCH:
		// no such process
		return false, nil
	case syscall.EPERM:
		// operation not permitted, but process exist
		return true, nil
	}
	// other signal: process not exist
	return false, nil
}
