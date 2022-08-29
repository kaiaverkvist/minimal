package setup

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

func Start(e *echo.Echo, port string, autoTls bool, cert string, pkey string, domains []string) {
	if autoTls {
		startAutoTLS(e, port, cert, pkey, domains)
		return
	}

	startInsecure(e, port)
	return
}

func startInsecure(e *echo.Echo, port string) {
	err := e.Start(port)
	if err != nil {
		log.Error("Unable to start server in insecure mode > ", err)
	}
}

func startAutoTLS(e *echo.Echo, port string, cert string, pkey string, domains []string) {
	dirCache := autocert.DirCache("/var/www/.cache")
	e.AutoTLSManager.Cache = dirCache
	autoTLSManager := autocert.Manager{
		Prompt: autocert.AcceptTOS,
		// Cache certificates to avoid issues with rate limits (https://letsencrypt.org/docs/rate-limits)
		Cache:      dirCache,
		HostPolicy: autocert.HostWhitelist(domains...),
	}
	s := http.Server{
		Addr:    port,
		Handler: e,
		TLSConfig: &tls.Config{
			GetCertificate: autoTLSManager.GetCertificate,
			NextProtos:     []string{acme.ALPNProto},
		},
		ReadTimeout: 30 * time.Second,
	}

	if err := s.ListenAndServeTLS(cert, pkey); err != http.ErrServerClosed {
		e.Logger.Fatal("Unable to start server in AutoTLS mode > ", err)
	}
}
