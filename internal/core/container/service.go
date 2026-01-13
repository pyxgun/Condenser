package container

import (
	"condenser/internal/env"
	"condenser/internal/runtime"
	"condenser/internal/runtime/droplet"
	"condenser/internal/store/ipam"
	"condenser/internal/utils"
	"fmt"
	"path/filepath"
	"strings"
)

func NewContaierService() *ContainerService {
	return &ContainerService{
		filesystemHandler: utils.NewFilesystemExecutor(),
		commandFactory:    utils.NewCommandFactory(),
		runtimeHandler:    droplet.NewDropletHandler(),
		ipamStoreHandler:  ipam.NewIpamStore(env.IpamStorePath),
		ipamHandler:       ipam.NewIpamManager(ipam.NewIpamStore(env.IpamStorePath)),
	}
}

type ContainerService struct {
	filesystemHandler utils.FilesystemHandler
	commandFactory    utils.CommandFactory
	runtimeHandler    runtime.RuntimeHandler
	ipamStoreHandler  ipam.IpamStoreHandler
	ipamHandler       ipam.IpamHandler
}

// == service: create ==
func (s *ContainerService) Create(createParameter ServiceCreateModel) (string, error) {
	// 1. generate container id
	containerId := utils.NewUlid()

	// 2. setup container directory
	if err := s.setupContainerDirectory(containerId); err != nil {
		return "", fmt.Errorf("create container directory failed: %w", err)
	}

	// 3. setup etc files
	if err := s.setupEtcFiles(containerId); err != nil {
		return "", fmt.Errorf("setup etc files failed: %w", err)
	}

	// 4. setup cgroup subtree
	if err := s.setupCgroupSubtree(containerId); err != nil {
		return "", fmt.Errorf("setup cgroup subtree failed: %w", err)
	}

	// 5. create spec (config.json)
	if err := s.createContainerSpec(containerId, createParameter.Image, createParameter.Command); err != nil {
		return "", fmt.Errorf("create spec failed: %w", err)
	}

	// 6. create container
	if err := s.createContainer(containerId); err != nil {
		return "", fmt.Errorf("create container failed: %w", err)
	}

	return containerId, nil
}

func (s *ContainerService) setupContainerDirectory(containerId string) error {
	containerDir := filepath.Join(env.ContainerRootDir, containerId)
	dirs := []string{
		containerDir,
		filepath.Join(containerDir, "diff"),
		filepath.Join(containerDir, "work"),
		filepath.Join(containerDir, "merged"),
		filepath.Join(containerDir, "etc"),
	}
	for _, dir := range dirs {
		if err := s.filesystemHandler.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (s *ContainerService) setupEtcFiles(containerId string) error {
	etcDir := filepath.Join(env.ContainerRootDir, containerId, "etc")

	// /etc/hosts
	hostsPath := filepath.Join(etcDir, "hosts")
	hostsData := "127.0.0.1 localhost\n"
	if err := s.filesystemHandler.WriteFile(hostsPath, []byte(hostsData), 0o644); err != nil {
		return err
	}

	// /etc/hostname
	hostnamePath := filepath.Join(etcDir, "hostname")
	hostnameData := fmt.Sprintf("%s\n", containerId)
	if err := s.filesystemHandler.WriteFile(hostnamePath, []byte(hostnameData), 0o644); err != nil {
		return err
	}

	// /etc/resolv.conf
	resolvPath := filepath.Join(etcDir, "resolv.conf")
	resolvData := "nameserver 8.8.8.8\n"
	if err := s.filesystemHandler.WriteFile(resolvPath, []byte(resolvData), 0o644); err != nil {
		return err
	}

	return nil
}

func (s *ContainerService) setupCgroupSubtree(containerId string) error {
	cgroupPath := filepath.Join(env.CgroupRuntimeDir, containerId)

	if err := s.filesystemHandler.MkdirAll(cgroupPath, 0o755); err != nil {
		return err
	}
	return nil
}

func (s *ContainerService) createContainerSpec(containerId string, image string, command []string) error {
	// spec parametr
	// rootfs
	rootfs := filepath.Join(env.ContainerRootDir, containerId, "merged")

	// cwd
	cwd := "/" // TODO: retrieve from image bundle

	// command
	var cmd string
	if len(command) != 0 {
		cmd = strings.Join(command, " ")
	} else {
		cmd = "/bin/sh" // TODO: retrieve from image bundle
	}

	// namespace
	namespace := []string{"mount", "network", "uts", "pid", "ipc", "user", "cgroup"}

	// hostname
	hostname := containerId

	// host interface
	hostInterface, err := s.ipamStoreHandler.GetDefaultInterface()
	if err != nil {
		return err
	}

	// bridge interface
	// TODO: network option for specifing subnet
	bridgeInterface := "raind0"

	// container interface
	containerInterface := "eth0"
	// container interface address
	containerInterfaceAddr, err := s.ipamHandler.Allocate(containerId, bridgeInterface)
	if err != nil {
		return err
	}
	containerInterfaceAddr = containerInterfaceAddr + "/24"
	// container gateway
	// TODO: network option for specifing subnet
	containerGateway := strings.Split("10.166.0.254/24", "/")[0]
	// container dns
	containerDns := []string{"8.8.8.8"}

	// TODO: image layers retrieve from image bundle
	imageLayer := []string{filepath.Join(env.LayerRootDir, image)}
	upperDir := filepath.Join(env.ContainerRootDir, containerId, "diff")
	workDir := filepath.Join(env.ContainerRootDir, containerId, "work")
	outputDir := filepath.Join(env.ContainerRootDir, containerId)

	// hook
	hookAddr, err := s.ipamStoreHandler.GetDefaultInterfaceAddr()
	if err != nil {
		return err
	}
	hookAddr = strings.Split(hookAddr, "/")[0]
	createRuntimeHook := []string{
		strings.Join([]string{
			"/usr/bin/curl", "-sS", "-X", "POST",
			"--fail-with-body", "--connect-timeout", "1", "--max-time", "2",
			"-H", "Content-Type: application/json", "-H", "X-Hook-Event: createRuntime",
			"--data-binary", "@-",
			"http://" + hookAddr + ":7756/v1/hooks/droplet",
		}, ","),
	}
	createContainerHook := []string{
		strings.Join([]string{
			"/usr/bin/curl", "-sS", "-X", "POST",
			"--fail-with-body", "--connect-timeout", "1", "--max-time", "2",
			"-H", "Content-Type: application/json", "-H", "X-Hook-Event: createContainer",
			"--data-binary", "@-",
			"http://" + hookAddr + ":7756/v1/hooks/droplet",
		}, ","),
	}
	poststartHook := []string{
		strings.Join([]string{
			"/usr/bin/curl", "-sS", "-X", "POST",
			"--fail-with-body", "--connect-timeout", "1", "--max-time", "2",
			"-H", "Content-Type: application/json", "-H", "X-Hook-Event: poststart",
			"--data-binary", "@-",
			"http://" + hookAddr + ":7756/v1/hooks/droplet",
		}, ","),
	}
	stopContainerHook := []string{
		strings.Join([]string{
			"/usr/bin/curl", "-sS", "-X", "POST",
			"--fail-with-body", "--connect-timeout", "1", "--max-time", "2",
			"-H", "Content-Type: application/json", "-H", "X-Hook-Event: stopContainer",
			"--data-binary", "@-",
			"http://" + hookAddr + ":7756/v1/hooks/droplet",
		}, ","),
	}
	poststopHook := []string{
		strings.Join([]string{
			"/usr/bin/curl", "-sS", "-X", "POST",
			"--fail-with-body", "--connect-timeout", "1", "--max-time", "2",
			"-H", "Content-Type: application/json", "-H", "X-Hook-Event: poststop",
			"--data-binary", "@-",
			"http://" + hookAddr + ":7756/v1/hooks/droplet",
		}, ","),
	}

	specParameter := runtime.SpecModel{
		Rootfs:                 rootfs,
		Cwd:                    cwd,
		Command:                cmd,
		Namespace:              namespace,
		Hostname:               hostname,
		HostInterface:          hostInterface,
		BridgeInterface:        bridgeInterface,
		ContainerInterface:     containerInterface,
		ContainerInterfaceAddr: containerInterfaceAddr,
		ContainerGateway:       containerGateway,
		ContainerDns:           containerDns,
		ImageLayer:             imageLayer,
		UpperDir:               upperDir,
		WorkDir:                workDir,
		CreateRuntimeHook:      createRuntimeHook,
		CreateContainerHook:    createContainerHook,
		PoststartHook:          poststartHook,
		StopContainerHook:      stopContainerHook,
		PoststopHook:           poststopHook,
		Output:                 outputDir,
	}

	// runtime: spec
	if err := s.runtimeHandler.Spec(specParameter); err != nil {
		return err
	}

	return nil
}

func (s *ContainerService) createContainer(containerId string) error {
	// runtime: create
	if err := s.runtimeHandler.Create(runtime.CreateModel{ContainerId: containerId}); err != nil {
		return err
	}
	return nil
}

// ===========

// == service: start ==
func (s *ContainerService) Start(startParameter ServiceStartModel) (string, error) {
	// start container
	if err := s.startContainer(startParameter.ContainerId, startParameter.Interactive); err != nil {
		return "", fmt.Errorf("start container failed: %w", err)
	}
	return startParameter.ContainerId, nil
}

func (s *ContainerService) startContainer(containerId string, interactive bool) error {
	// runtime: start
	if err := s.runtimeHandler.Start(
		runtime.StartModel{
			ContainerId: containerId,
			Interactive: interactive,
		},
	); err != nil {
		return err
	}

	return nil
}

// =====================

// == service: delete ==
func (s *ContainerService) Delete(deleteParameter ServiceDeleteModel) (string, error) {
	// 1. delete container
	if err := s.deleteContainer(deleteParameter.ContainerId); err != nil {
		return "", fmt.Errorf("delete container failed: %w", err)
	}

	// 2. release address
	if err := s.releaseAddress(deleteParameter.ContainerId); err != nil {
		return "", fmt.Errorf("release address failed: %w", err)
	}

	// 3. delete container directory
	if err := s.deleteContainerDirectory(deleteParameter.ContainerId); err != nil {
		return "", fmt.Errorf("delete container directory failed: %w", err)
	}

	// 4. delete cgroup subtree
	if err := s.deleteCgroupSubtree(deleteParameter.ContainerId); err != nil {
		return "", fmt.Errorf("delete cgroup subtree failed: %w", err)
	}

	return deleteParameter.ContainerId, nil
}

func (s *ContainerService) deleteContainer(containerId string) error {
	// runtime: delete
	if err := s.runtimeHandler.Delete(
		runtime.DeleteModel{
			ContainerId: containerId,
		},
	); err != nil {
		return err
	}
	return nil
}

func (s *ContainerService) releaseAddress(containerId string) error {
	if err := s.ipamHandler.Release(containerId); err != nil {
		return err
	}
	return nil
}

func (s *ContainerService) deleteContainerDirectory(containerId string) error {
	containerDir := filepath.Join(env.ContainerRootDir, containerId)
	if err := s.filesystemHandler.RemoveAll(containerDir); err != nil {
		return err
	}
	return nil
}

func (s *ContainerService) deleteCgroupSubtree(containerId string) error {
	cgroupPath := filepath.Join(env.CgroupRuntimeDir, containerId)
	if err := s.filesystemHandler.Remove(cgroupPath); err != nil {
		return err
	}
	return nil
}

// =====================

// == service: stop ==
func (s *ContainerService) Stop(stopParameter ServiceStopModel) (string, error) {
	// stop container
	if err := s.stopContainer(stopParameter.ContainerId); err != nil {
		return "", fmt.Errorf("stop failed: %w", err)
	}
	return stopParameter.ContainerId, nil
}

func (s *ContainerService) stopContainer(containerId string) error {
	// runtime: stop
	if err := s.runtimeHandler.Stop(
		runtime.StopModel{
			ContainerId: containerId,
		},
	); err != nil {
		return err
	}
	return nil
}

// ===================
