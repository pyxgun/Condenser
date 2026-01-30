package monitor

import (
	"condenser/internal/store/csm"
	"condenser/internal/utils"
	"context"
	"log"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
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
	resolver := NewResolver(m.csmHandler)
	// watch CSM file update
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := resolver.Watch(ctx); err != nil {
			log.Printf("watch stopped: %v", err)
		}
	}()

	for {
		time.Sleep(100 * time.Millisecond)

		for _, container := range resolver.ResolveMap {
			// status check
			// monitoring target: created, running
			if container.Status != "running" && container.Status != "created" {
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

func NewResolver(csmHandler csm.CsmHandler) *Resolver {
	resolver := &Resolver{
		ResolveMap: map[string]ContainerMeta{},
		csmHandler: csmHandler,
	}
	containerList, _ := csmHandler.GetContainerList()
	for _, c := range containerList {
		if _, ok := resolver.ResolveMap[c.ContainerId]; !ok {
			resolver.ResolveMap[c.ContainerId] = ContainerMeta{
				ContainerId:   c.ContainerId,
				ContainerName: c.ContainerName,
				SpiffeId:      c.SpiffeId,
				Status:        c.State,
				Pid:           c.Pid,
			}
		}
	}
	return resolver
}

type Resolver struct {
	ResolveMap map[string]ContainerMeta
	csmHandler csm.CsmHandler
}

func (r *Resolver) Refresh() {
	r.ResolveMap = map[string]ContainerMeta{}
	containerList, _ := r.csmHandler.GetContainerList()
	for _, c := range containerList {
		if _, ok := r.ResolveMap[c.ContainerId]; !ok {
			r.ResolveMap[c.ContainerId] = ContainerMeta{
				ContainerId:   c.ContainerId,
				ContainerName: c.ContainerName,
				SpiffeId:      c.SpiffeId,
				Status:        c.State,
				Pid:           c.Pid,
			}
		}
	}
}

func (r *Resolver) Watch(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	dir := filepath.Dir(utils.CsmStorePath)
	base := filepath.Base(utils.CsmStorePath)

	if err := w.Add(dir); err != nil {
		return err
	}

	var pending atomic.Bool
	trigger := func() {
		if pending.CompareAndSwap(false, true) {
			go func() {
				time.Sleep(50 * time.Millisecond)
				r.Refresh()
				pending.Store(false)
			}()
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev := <-w.Events:
			if filepath.Base(ev.Name) != base {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
				trigger()
			}
		case <-w.Errors:
		}
	}
}
