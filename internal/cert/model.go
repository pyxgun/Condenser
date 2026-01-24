package cert

import (
	"net"
	"time"
)

type CertConfig struct {
	CommonName  string
	DNSNames    []string
	IPAddresses []net.IP
	ValidFor    time.Duration
}

type ClientCertConfig struct {
	CommonName string
	DNSNames   []string
	URISANs    []string
	ValidFor   time.Duration
}
