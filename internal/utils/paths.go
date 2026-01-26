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
	NpmStorePath  = "/etc/raind/store/npm.json"

	CgroupRuntimeDir         = "/sys/fs/cgroup/raind"
	CgroupSubtreeControlPath = "/sys/fs/cgroup/raind/cgroup.subtree_control"

	CertDir                = "/etc/raind/cert"
	PublicCertPath         = "/etc/raind/cert/raind.crt"
	PrivateKeyPath         = "/etc/raind/cert/raind.key"
	ClientIssuerCACertPath = "/etc/raind/cert/raindClientCA.crt"
	ClientIssuerCAKeyPath  = "/etc/raind/cert/raindClientCA.key"
	ClientCertPath         = "/etc/raind/cert/raindClient.crt"
	ClientKeyPath          = "/etc/raind/cert/raindClient.key"
	HookClientCertPath     = "/etc/raind/cert/raindHookClient.crt"
	HookClientKeyPath      = "/etc/raind/cert/raindHookClient.key"
)
