package container

import (
	"condenser/internal/core/image"
	"condenser/internal/core/network"
	"condenser/internal/runtime"
	"condenser/internal/runtime/droplet"
	"condenser/internal/store/csm"
	"condenser/internal/store/ilm"
	"condenser/internal/store/ipam"
	"condenser/internal/utils"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"al.essio.dev/pkg/shellescape"
)

func NewContaierService() *ContainerService {
	return &ContainerService{
		filesystemHandler: utils.NewFilesystemExecutor(),
		commandFactory:    utils.NewCommandFactory(),
		runtimeHandler:    droplet.NewDropletHandler(),

		ipamHandler: ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath)),
		ilmHandler:  ilm.NewIlmManager(ilm.NewIlmStore(utils.IlmStorePath)),
		csmHandler:  csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),

		imageServiceHandler:   image.NewImageService(),
		networkServiceHandler: network.NewNetworkService(),
	}
}

type ContainerService struct {
	filesystemHandler utils.FilesystemHandler
	commandFactory    utils.CommandFactory
	runtimeHandler    runtime.RuntimeHandler

	ipamHandler ipam.IpamHandler
	ilmHandler  ilm.IlmHandler
	csmHandler  csm.CsmHandler

	imageServiceHandler   image.ImageServiceHandler
	networkServiceHandler network.NetworkServiceHandler
}

// == service: create ==
func (s *ContainerService) Create(createParameter ServiceCreateModel) (id string, err error) {
	// 1. generate container id and name
	containerId := utils.NewUlid()[:12]
	//    if name is not set, generate a random name
	containerName := createParameter.Name
	if containerName == "" {
		containerName, err = s.generateContainerName()
		if err != nil {
			return "", err
		}
	} else {
		// validate the name is not used by other container
		if s.csmHandler.IsNameAlreadyUsed(containerName) {
			return "", fmt.Errorf("name: %s already used by other container", containerName)
		}
	}

	// RollbackFlag for handling rollback handling when process is not completed successfuly
	var rollbackFlag RollbackFlag
	defer func() {
		if err != nil {
			if rbErr := s.rollback(rollbackFlag, containerId); rbErr != nil {
				err = rbErr
			}
		}
	}()

	// 2. check if the requested image exist
	imageRepo, imageRef, err := s.parseImageRef(createParameter.Image)
	if err != nil {
		return "", err
	}

	// 3. if the image not exist in local, pull image
	if !s.ilmHandler.IsImageExist(imageRepo, imageRef) {
		if err := s.pullImage(createParameter.Image, createParameter.Os, createParameter.Arch); err != nil {
			return "", err
		}
	}

	// 4. load image config file
	imageConfigPath, err := s.ilmHandler.GetConfigPath(imageRepo, imageRef)
	if err != nil {
		return "", err
	}
	imageConfig, err := s.imageServiceHandler.GetImageConfig(imageConfigPath)
	if err != nil {
		return "", err
	}

	// 5. allocate address
	bridgeInterface := createParameter.Network
	if bridgeInterface == "" {
		bridgeInterface = "raind0"
	}
	containerGateway, containerAddr, err := s.allocateAddress(containerId, bridgeInterface)
	if err != nil {
		return "", err
	}
	rollbackFlag.AllocateAddr = true

	// 6. create CSM entry with state=creating, pid=0, creatingAt=nil
	//    command=if user specified, use it. if not, use image config's command
	var command []string
	if len(createParameter.Command) > 0 {
		command = createParameter.Command
	} else {
		command = slices.Concat(imageConfig.Config.Entrypoint, imageConfig.Config.Cmd)
	}
	if err := s.csmHandler.StoreContainer(containerId, "creating", 0, imageRepo, imageRef, command, containerName); err != nil {
		return "", err
	}
	rollbackFlag.CSMEntry = true

	// 7. setup container directory
	if err := s.setupContainerDirectory(containerId); err != nil {
		return "", fmt.Errorf("create container directory failed: %w", err)
	}
	rollbackFlag.DirectoryEnv = true

	// 8. setup etc files
	if err := s.setupEtcFiles(containerId, containerAddr); err != nil {
		return "", fmt.Errorf("setup etc files failed: %w", err)
	}

	// 9. setup cgroup subtree
	if err := s.setupCgroupSubtree(containerId); err != nil {
		return "", fmt.Errorf("setup cgroup subtree failed: %w", err)
	}
	rollbackFlag.CgroupEntry = true

	// 10. create spec (config.json)
	if err := s.createContainerSpec(
		containerId, createParameter, imageRepo, imageRef, imageConfig,
		bridgeInterface, containerAddr, containerGateway,
	); err != nil {
		return "", fmt.Errorf("create spec failed: %w", err)
	}

	// 11. setup forward rule
	if err := s.setupForwardRule(containerId, createParameter.Port); err != nil {
		return "", fmt.Errorf("forward rule failed: %w", err)
	}
	rollbackFlag.ForwardRule = true

	// 12. create container
	if err := s.createContainer(containerId, createParameter.Tty); err != nil {
		return "", fmt.Errorf("create container failed: %w", err)
	}

	return containerId, nil
}

type RollbackFlag struct {
	AllocateAddr bool
	CSMEntry     bool
	DirectoryEnv bool
	CgroupEntry  bool
	ForwardRule  bool
}

func (s *ContainerService) rollback(rollbackFlag RollbackFlag, containerId string) error {
	if rollbackFlag.AllocateAddr {
		if err := s.releaseAddress(containerId); err != nil {
			return err
		}
	}
	if rollbackFlag.CSMEntry {
		if err := s.csmHandler.RemoveContainer(containerId); err != nil {
			return err
		}
	}
	if rollbackFlag.DirectoryEnv {
		if err := s.deleteContainerDirectory(containerId); err != nil {
			return err
		}
	}
	if rollbackFlag.CgroupEntry {
		if err := s.deleteCgroupSubtree(containerId); err != nil {
			return err
		}
	}
	if rollbackFlag.ForwardRule {
		if err := s.cleanupForwardRules(containerId); err != nil {
			return err
		}
	}
	return nil
}

func (s *ContainerService) generateContainerName() (string, error) {
	genCount := 0
	for {
		randName, err := utils.GenerateRandName()
		if err != nil {
			return "", err
		}
		if !s.csmHandler.IsNameAlreadyUsed(randName) {
			return randName, nil
		}
		if genCount >= 10 {
			return "", fmt.Errorf("cannot assign random name")
		}
		genCount++
	}
}

func (s *ContainerService) pullImage(targetImage string, os string, arch string) error {
	var (
		targetOs   string
		targetArch string
	)
	if os == "" {
		targetOs = utils.HostOs()
	}
	if arch == "" {
		hostArch, err := utils.HostArch()
		if err != nil {
			return err
		}
		targetArch = hostArch
	}
	if err := s.imageServiceHandler.Pull(image.ServicePullModel{
		Image: targetImage,
		Os:    targetOs,
		Arch:  targetArch,
	}); err != nil {
		return err
	}
	return nil
}

func (s *ContainerService) setupContainerDirectory(containerId string) error {
	containerDir := filepath.Join(utils.ContainerRootDir, containerId)
	dirs := []string{
		containerDir,
		filepath.Join(containerDir, "diff"),
		filepath.Join(containerDir, "work"),
		filepath.Join(containerDir, "merged"),
		filepath.Join(containerDir, "etc"),
		filepath.Join(containerDir, "logs"),
		filepath.Join(containerDir, "cert"),
	}
	for _, dir := range dirs {
		if err := s.filesystemHandler.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (s *ContainerService) setupEtcFiles(containerId string, containerAddr string) error {
	etcDir := filepath.Join(utils.ContainerRootDir, containerId, "etc")

	// /etc/hosts
	hostsPath := filepath.Join(etcDir, "hosts")
	hostsData := fmt.Sprintf("127.0.0.1 localhost\n%s %s\n", strings.SplitN(containerAddr, "/", 2)[0], containerId)
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
	cgroupPath := filepath.Join(utils.CgroupRuntimeDir, containerId)

	if err := s.filesystemHandler.MkdirAll(cgroupPath, 0o755); err != nil {
		return err
	}
	return nil
}

func (s *ContainerService) ChangeCgroupMode(containerId string) error {
	cgroupPath := filepath.Join(utils.CgroupRuntimeDir, containerId)

	if err := s.filesystemHandler.Chmod(cgroupPath, 0o555); err != nil {
		return err
	}
	return nil
}

func (s *ContainerService) allocateAddress(containerId string, bridgeInterface string) (string, string, error) {
	containerInterfaceAddr, err := s.ipamHandler.Allocate(containerId, bridgeInterface)
	if err != nil {
		return "", "", err
	}
	containerInterfaceAddr = containerInterfaceAddr + "/24"
	// container gateway
	containerGateway, err := s.ipamHandler.GetBridgeAddr(bridgeInterface)
	if err != nil {
		return "", "", err
	}
	containerGateway = strings.Split(containerGateway, "/")[0]

	return containerGateway, containerInterfaceAddr, nil
}

func (s *ContainerService) createContainerSpec(
	containerId string, createParameter ServiceCreateModel,
	imageRepo, imageRef string, imageConfig image.ImageConfigFile,
	bridge, containerAddr, containerGateway string,
) error {

	// spec parametr
	// rootfs
	rootfs := filepath.Join(utils.ContainerRootDir, containerId, "merged")

	// cwd
	cwd := imageConfig.Config.WorkingDir
	if cwd == "" {
		cwd = "/"
	}

	// command
	var cmd string
	if len(createParameter.Command) != 0 {
		cmd = s.buildCommand(createParameter.Command, []string{})
	} else {
		cmd = s.buildCommand(imageConfig.Config.Entrypoint, imageConfig.Config.Cmd)
	}

	// namespace
	namespace := []string{"mount", "network", "uts", "pid", "ipc", "user", "cgroup"}

	// hostname
	hostname := containerId

	// env
	envs := imageConfig.Config.Env

	// mount
	mount := createParameter.Mount

	// host interface
	hostInterface, err := s.ipamHandler.GetDefaultInterface()
	if err != nil {
		return err
	}

	// container interface
	containerInterface := "rd_" + containerId
	containerDns := []string{"8.8.8.8"}

	imageLayer, err := s.ilmHandler.GetRootfsPath(imageRepo, imageRef)
	if err != nil {
		return err
	}
	upperDir := filepath.Join(utils.ContainerRootDir, containerId, "diff")
	workDir := filepath.Join(utils.ContainerRootDir, containerId, "work")
	outputDir := filepath.Join(utils.ContainerRootDir, containerId)

	// hook
	hookAddr, err := s.ipamHandler.GetDefaultInterfaceAddr()
	if err != nil {
		return err
	}
	hookAddr = strings.Split(hookAddr, "/")[0]
	createRuntimeHook := []string{
		strings.Join([]string{
			"/usr/bin/raind-hook",
			"--url", "https://" + hookAddr + ":7757/v1/pki/sign",
			"--event", "requestCert",
			"--ca", utils.PublicCertPath,
			"--cert", utils.HookClientCertPath,
			"--key", utils.HookClientKeyPath,
		}, ","),
		strings.Join([]string{
			"/usr/bin/raind-hook",
			"--url", "https://" + hookAddr + ":7756/v1/hooks/droplet",
			"--event", "createRuntime",
			"--ca", utils.PublicCertPath,
			"--cert", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.crt"),
			"--key", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.key"),
		}, ","),
	}
	createRuntimeHookEnv := []string{
		"RAIND-HOOK-SETTER=CONDENSER",
		"RAIND-HOOK-SETTER=CONDENSER",
	}
	createContainerHook := []string{
		strings.Join([]string{
			"/usr/bin/raind-hook",
			"--url", "https://" + hookAddr + ":7756/v1/hooks/droplet",
			"--event", "createContainer",
			"--ca", utils.PublicCertPath,
			"--cert", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.crt"),
			"--key", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.key"),
		}, ","),
	}
	createContainerHookEnv := []string{
		"RAIND-HOOK-SETTER=CONDENSER",
	}
	poststartHook := []string{
		strings.Join([]string{
			"/usr/bin/raind-hook",
			"--url", "https://" + hookAddr + ":7756/v1/hooks/droplet",
			"--event", "poststart",
			"--ca", utils.PublicCertPath,
			"--cert", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.crt"),
			"--key", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.key"),
		}, ","),
	}
	poststartHookEnv := []string{
		"RAIND-HOOK-SETTER=CONDENSER",
	}
	stopContainerHook := []string{
		strings.Join([]string{
			"/usr/bin/raind-hook",
			"--url", "https://" + hookAddr + ":7756/v1/hooks/droplet",
			"--event", "stopContainer",
			"--ca", utils.PublicCertPath,
			"--cert", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.crt"),
			"--key", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.key"),
		}, ","),
	}
	stopContainerHookEnv := []string{
		"RAIND-HOOK-SETTER=CONDENSER",
	}
	poststopHook := []string{
		strings.Join([]string{
			"/usr/bin/raind-hook",
			"--url", "https://" + hookAddr + ":7756/v1/hooks/droplet",
			"--event", "poststop",
			"--ca", utils.PublicCertPath,
			"--cert", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.crt"),
			"--key", filepath.Join(utils.ContainerRootDir, containerId, "/cert/client.key"),
		}, ","),
	}
	poststopHookEnv := []string{
		"RAIND-HOOK-SETTER=CONDENSER",
	}

	specParameter := runtime.SpecModel{
		Rootfs:                 rootfs,
		Cwd:                    cwd,
		Command:                cmd,
		Namespace:              namespace,
		Hostname:               hostname,
		Env:                    envs,
		Mount:                  mount,
		HostInterface:          hostInterface,
		BridgeInterface:        bridge,
		ContainerInterface:     containerInterface,
		ContainerInterfaceAddr: containerAddr,
		ContainerGateway:       containerGateway,
		ContainerDns:           containerDns,
		ImageLayer:             []string{imageLayer},
		UpperDir:               upperDir,
		WorkDir:                workDir,
		CreateRuntimeHook:      createRuntimeHook,
		CreateRuntimeHookEnv:   createRuntimeHookEnv,
		CreateContainerHook:    createContainerHook,
		CreateContainerHookEnv: createContainerHookEnv,
		PoststartHook:          poststartHook,
		PoststartHookEnv:       poststartHookEnv,
		StopContainerHook:      stopContainerHook,
		StopContainerHookEnv:   stopContainerHookEnv,
		PoststopHook:           poststopHook,
		PoststopHookEnv:        poststopHookEnv,
		Output:                 outputDir,
	}

	// runtime: spec
	if err := s.runtimeHandler.Spec(specParameter); err != nil {
		return err
	}

	return nil
}

func (s *ContainerService) createContainer(containerId string, tty bool) error {
	// runtime: create
	if err := s.runtimeHandler.Create(runtime.CreateModel{ContainerId: containerId, Tty: tty}); err != nil {
		return err
	}
	return nil
}

func (s *ContainerService) parseImageRef(imageStr string) (repository, reference string, err error) {
	// image string pattern
	// - ubuntu 				-> library/ubuntu:latest
	// - ubuntu:24.04 			-> library/ubuntu:24.04
	// - library/ubuntu:24.04 	-> library/ubuntu:24.04
	// - nginx@sha256:... 		-> library/nginx@sha256:...

	var repo, ref string
	if strings.Contains(imageStr, "@") {
		parts := strings.SplitN(imageStr, "@", 2)
		repo, ref = parts[0], parts[1]
	} else {
		parts := strings.SplitN(imageStr, ":", 2)
		repo = parts[0]
		if len(parts) == 2 && parts[1] != "" {
			ref = parts[1]
		} else {
			ref = "latest"
		}
	}

	if repo == "" {
		return "", "", errors.New("empty repository")
	}
	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}
	return repo, ref, nil
}

func (s *ContainerService) buildCommand(entrypoint, cmd []string) string {
	var all []string
	all = append(all, entrypoint...)
	all = append(all, cmd...)

	var quoted []string
	for _, a := range all {
		quoted = append(quoted, shellescape.Quote(a))
	}
	return strings.Join(quoted, " ")
}

func (s *ContainerService) setupForwardRule(containerId string, ports []string) error {
	if len(ports) == 0 {
		return nil
	}

	// create forward rule
	for _, port := range ports {
		var (
			sport    string
			dport    string
			protocol string
		)
		portParts := strings.Split(port, ":")
		if len(portParts) == 2 {
			sport = portParts[0]
			dport = portParts[1]
			protocol = "tcp"
		} else if len(portParts) == 3 {
			sport = portParts[0]
			dport = portParts[1]
			protocol = portParts[2]
		} else {
			return fmt.Errorf("port format failed: %s", port)
		}

		if err := s.networkServiceHandler.CreateForwardingRule(
			containerId,
			network.ServiceNetworkModel{
				HostPort:      sport,
				ContainerPort: dport,
				Protocol:      protocol,
			},
		); err != nil {
			return err
		}

		// update ipam
		iSport, _ := strconv.Atoi(sport)
		iDport, _ := strconv.Atoi(dport)
		if err := s.ipamHandler.SetForwardInfo(containerId, iSport, iDport, protocol); err != nil {
			return err
		}
	}

	return nil
}

// ===========

func (s *ContainerService) getContainerState(containerId string) (string, error) {
	containerInfo, err := s.csmHandler.GetContainerById(containerId)
	if err != nil {
		return "", err
	}
	return containerInfo.State, nil
}

// == service: start ==
func (s *ContainerService) Start(startParameter ServiceStartModel) (string, error) {
	// resolve container id
	containerId, err := s.csmHandler.ResolveContainerId(startParameter.ContainerId)
	if err != nil {
		return "", fmt.Errorf("container: %s not found", startParameter.ContainerId)
	}

	containerState, err := s.getContainerState(containerId)
	if err != nil {
		return "", err
	}

	switch containerState {
	case "created":
		// start container
		if err := s.startContainer(containerId, startParameter.Tty); err != nil {
			return "", fmt.Errorf("start container failed: %w", err)
		}

	case "running":
		// already started. ignore operation
		return "", fmt.Errorf("container: %s already started", containerId)

	case "stopped":
		// create container
		if err := s.createContainer(containerId, startParameter.Tty); err != nil {
			return "", fmt.Errorf("start container failed: %w", err)
		}
		// start container
		if err := s.startContainer(containerId, startParameter.Tty); err != nil {
			return "", fmt.Errorf("start container failed: %w", err)
		}

	default:
		return "", fmt.Errorf("start operation not allowed to current container status: %s", containerState)
	}

	return containerId, nil
}

func (s *ContainerService) startContainer(containerId string, tty bool) error {
	// runtime: start
	if err := s.runtimeHandler.Start(
		runtime.StartModel{
			ContainerId: containerId,
			Tty:         tty,
		},
	); err != nil {
		return err
	}

	return nil
}

// =====================

// == service: delete ==
func (s *ContainerService) Delete(deleteParameter ServiceDeleteModel) (string, error) {
	// resolve container id
	containerId, err := s.csmHandler.ResolveContainerId(deleteParameter.ContainerId)
	if err != nil {
		return "", fmt.Errorf("container: %s not found", deleteParameter.ContainerId)
	}

	containerState, err := s.getContainerState(containerId)
	if err != nil {
		return "", err
	}

	switch containerState {
	case "creating", "created", "stopped":
		// 1. delete container
		if err := s.deleteContainer(containerId); err != nil {
			return "", fmt.Errorf("delete container failed: %w", err)
		}

		// 2. cleanup forward rule
		if err := s.cleanupForwardRules(containerId); err != nil {
			return "", fmt.Errorf("cleanup forward rule failed: %w", err)
		}

		// 2. release address
		if err := s.releaseAddress(containerId); err != nil {
			return "", fmt.Errorf("release address failed: %w", err)
		}

		// 3. delete container directory
		if err := s.deleteContainerDirectory(containerId); err != nil {
			return "", fmt.Errorf("delete container directory failed: %w", err)
		}

		// 4. delete cgroup subtree
		if err := s.deleteCgroupSubtree(containerId); err != nil {
			return "", fmt.Errorf("delete cgroup subtree failed: %w", err)
		}
	default:
		return "", fmt.Errorf("delete operation not allowed to current container status: %s", containerState)
	}

	return containerId, nil
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

func (s *ContainerService) cleanupForwardRules(containerId string) error {
	// retrieve network info
	forwards, err := s.ipamHandler.GetForwardInfo(containerId)
	if err != nil {
		return err
	}
	if len(forwards) == 0 {
		return nil
	}

	// remove rules
	for _, f := range forwards {
		if err := s.networkServiceHandler.RemoveForwardingRule(
			containerId,
			network.ServiceNetworkModel{
				HostPort:      strconv.Itoa(f.HostPort),
				ContainerPort: strconv.Itoa(f.ContainerPort),
				Protocol:      f.Protocol,
			},
		); err != nil {
			return err
		}
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
	containerDir := filepath.Join(utils.ContainerRootDir, containerId)
	if err := s.filesystemHandler.RemoveAll(containerDir); err != nil {
		return err
	}
	return nil
}

func (s *ContainerService) deleteCgroupSubtree(containerId string) error {
	cgroupPath := filepath.Join(utils.CgroupRuntimeDir, containerId)
	if err := s.filesystemHandler.Remove(cgroupPath); err != nil {
		return err
	}
	return nil
}

// =====================

// == service: stop ==
func (s *ContainerService) Stop(stopParameter ServiceStopModel) (string, error) {
	// resolve container id
	containerId, err := s.csmHandler.ResolveContainerId(stopParameter.ContainerId)
	if err != nil {
		return "", fmt.Errorf("container: %s not found", stopParameter.ContainerId)
	}

	containerState, err := s.getContainerState(containerId)
	if err != nil {
		return "", err
	}

	switch containerState {
	case "running":
		// stop container
		if err := s.stopContainer(containerId); err != nil {
			return "", fmt.Errorf("stop failed: %w", err)
		}
	default:
		return "", fmt.Errorf("stop operation not allowed to current container status: %s", containerState)
	}
	return containerId, nil
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

// == service: exec container ==
func (s *ContainerService) Exec(execParameter ServiceExecModel) error {
	// resolve container id
	containerId, err := s.csmHandler.ResolveContainerId(execParameter.ContainerId)
	if err != nil {
		return fmt.Errorf("container: %s not found", execParameter.ContainerId)
	}

	// runtime: exec
	if err := s.runtimeHandler.Exec(
		runtime.ExecModel{
			ContainerId: containerId,
			Tty:         execParameter.Tty,
			Entrypoint:  execParameter.Entrypoint,
		},
	); err != nil {
		return err
	}
	return nil
}

// =============================

// == service: get container list ==
func (s *ContainerService) GetContainerList() ([]ContainerState, error) {
	containerList, err := s.csmHandler.GetContainerList()
	if err != nil {
		return nil, err
	}
	poolList, err := s.ipamHandler.GetPoolList()
	if err != nil {
		return nil, err
	}

	var containerStateList []ContainerState
	for _, c := range containerList {
		var (
			forwards []ForwardInfo
			address  string
		)
		for _, p := range poolList {
			for addr, info := range p.Allocations {
				if info.ContainerId != c.ContainerId {
					continue
				}
				address = addr
				for _, f := range info.Forwards {
					forwards = append(forwards, ForwardInfo{
						HostPort:      f.HostPort,
						ContainerPort: f.ContainerPort,
						Protocol:      f.Protocol,
					})
				}
			}
		}

		containerStateList = append(containerStateList, ContainerState{
			ContainerId: c.ContainerId,
			Name:        c.ContainerName,
			State:       c.State,
			Pid:         c.Pid,
			Repository:  c.Repository,
			Reference:   c.Reference,
			Command:     c.Command,

			Address:  address,
			Forwards: forwards,

			CreatingAt: c.CreatingAt,
			CreatedAt:  c.CreatedAt,
			StartedAt:  c.StartedAt,
			StoppedAt:  c.StoppedAt,
		})
	}

	return containerStateList, nil
}

// =================================

// == service: get container by id ==
func (s *ContainerService) GetContainerById(containerId string) (ContainerState, error) {
	containerState, err := s.csmHandler.GetContainerById(containerId)
	if err != nil {
		return ContainerState{}, err
	}
	address, networkState, err := s.ipamHandler.GetNetworkInfoById(containerId)
	if err != nil {
		return ContainerState{}, err
	}

	var forwards []ForwardInfo
	for _, f := range networkState.Forwards {
		forwards = append(forwards, ForwardInfo{
			HostPort:      f.HostPort,
			ContainerPort: f.ContainerPort,
			Protocol:      f.Protocol,
		})
	}

	return ContainerState{
		ContainerId: containerState.ContainerId,
		State:       containerState.State,
		Pid:         containerState.Pid,
		Repository:  containerState.Repository,
		Reference:   containerState.Reference,
		Command:     containerState.Command,

		Address:  address,
		Forwards: forwards,

		CreatingAt: containerState.CreatingAt,
		CreatedAt:  containerState.CreatedAt,
		StartedAt:  containerState.StartedAt,
		StoppedAt:  containerState.StoppedAt,
	}, nil
}

// ==================================
