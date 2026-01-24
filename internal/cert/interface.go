package cert

type CertHandler interface {
	EnsureSelfSignedCert(certPath string, keyPath string, cfg CertConfig) error
	EnsureClientCACert(certPath string, keyPath string, cfg CertConfig) error
	IssueClientCert(certPath string, keyPath string, CACertPath string, CAKeyPath string, cfg ClientCertConfig) error
}
