package server

/*
 * generate a TLS configuration which conforms to best practices
 *
 * References:
 *	https://mozilla.github.io/server-side-tls/ssl-config-generator/
 *  https://wiki.mozilla.org/Security/Server_Side_TLS
 *  https://calomel.org/aesni_ssl_performance.html
 */

import (
	"crypto/tls"
)

// This configuration is for internal connections as the
// configuration is as strict as possible.

func tlsConfigFactory() *tls.Config {
	cfg := &tls.Config{
		PreferServerCipherSuites: true,             // don't let the client drive the cipher selection
		MinVersion:               tls.VersionTLS12, // only support TLS v1.2
		MaxVersion:               tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}

	return cfg
}
