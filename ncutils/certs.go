package ncutils

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"sync"
)

var (
	rootCertPool *x509.CertPool
	certOnce     sync.Once
)

// GetRootCAs loads root CA certificates from the system and Android certificate stores.
func GetRootCAs() *x509.CertPool {
	certOnce.Do(func() {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}

		caDirs := []string{
			"/apex/com.android.conscrypt/cacerts",
			"/system/etc/security/cacerts",
			"/system/etc/security/cacerts_google",
			"/data/misc/keychain/cacerts-added",
			"/data/misc/user/0/cacerts-added",
			"/data/adb/netclient/cacerts",
			"/etc/ssl/certs",
			"/etc/pki/tls/certs",
		}

		for _, dir := range caDirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if !entry.IsDir() {
					certPath := filepath.Join(dir, entry.Name())
					data, err := os.ReadFile(certPath)
					if err == nil && len(data) > 0 {
						pool.AppendCertsFromPEM(data)
					}
				}
			}
		}

		caFiles := []string{
			"/data/adb/netclient/ca-certificates.crt",
			"/etc/ssl/certs/ca-certificates.crt",
			"/etc/pki/tls/certs/ca-bundle.crt",
			"/etc/ssl/ca-bundle.pem",
		}

		for _, file := range caFiles {
			data, err := os.ReadFile(file)
			if err == nil && len(data) > 0 {
				pool.AppendCertsFromPEM(data)
			}
		}

		rootCertPool = pool
	})

	return rootCertPool
}
