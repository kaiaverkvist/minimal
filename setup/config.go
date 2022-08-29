package setup

type Config struct {
	DSN string

	HttpPort uint

	// Whether to use ACME auto-tls.
	AutoTLS bool

	CertKeyPath        string
	CertPrivateKeyPath string

	// FriendlyLogging makes logging look nice instead of wrapping it into JSON.
	FriendlyLogging bool

	Domains []string
}

var (
	DevelopmentConfig = Config{
		DSN:             "",
		HttpPort:        80,
		AutoTLS:         false,
		Domains:         []string{},
		FriendlyLogging: true,
	}
)
