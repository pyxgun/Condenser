package container

import (
	"condenser/internal/core/image"
	"condenser/internal/core/network"
	"condenser/internal/runtime"
	"condenser/internal/utils"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"al.essio.dev/pkg/shellescape"
)

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
			"/usr/local/bin/condenser-hook-agent",
			"--url", "https://localhost:7757/v1/pki/sign",
			"--event", "requestCert",
			"--ca", utils.PublicCertPath,
			"--cert", utils.HookClientCertPath,
			"--key", utils.HookClientKeyPath,
		}, ","),
		strings.Join([]string{
			"/usr/local/bin/condenser-hook-agent",
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
			"/usr/local/bin/condenser-hook-agent",
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
			"/usr/local/bin/condenser-hook-agent",
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
			"/usr/local/bin/condenser-hook-agent",
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
			"/usr/local/bin/condenser-hook-agent",
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
