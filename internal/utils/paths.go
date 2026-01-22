package utils

const (
	RootDir          = "/etc/raind"
	AuditLogDir      = "/etc/raind/log/"
	ContainerRootDir = "/etc/raind/container"
	ImageRootDir     = "/etc/raind/image"
	LayerRootDir     = "/etc/raind/image/layers"

	StoreDir      = "/etc/raind/store"
	IpamStorePath = "/etc/raind/store/ipam.json"
	CsmStorePath  = "/etc/raind/store/csm.json"
	IlmStorePath  = "/etc/raind/store/ilm.json"

	CgroupRuntimeDir         = "/sys/fs/cgroup/raind"
	CgroupSubtreeControlPath = "/sys/fs/cgroup/raind/cgroup.subtree_control"
)
