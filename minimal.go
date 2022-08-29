package minimal

import (
	"fmt"
	renderer "github.com/kaiaverkvist/echo-jet-template-renderer"
	"github.com/kaiaverkvist/minimal/database"
	"github.com/kaiaverkvist/minimal/server"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/css"
	"github.com/tdewolff/minify/html"
	"github.com/tdewolff/minify/js"
	"github.com/tdewolff/minify/json"
	"github.com/tdewolff/minify/svg"
	"github.com/tdewolff/minify/xml"
	"net/http"
	"regexp"
)

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

const (
	friendlyHeader = "⇨ ${time_rfc3339} (${short_file}:${line}) ${level}  "
	requestHeader  = "⇨ ${time_rfc3339} HTTP  ${method} ${uri} -> RESP ${status} (took ${latency_human}) (▼${bytes_in}B  ▲${bytes_out}B)\n"
)

type Provider interface {
	Register(e *echo.Echo)
}

type Server struct {
	e *echo.Echo

	// Routes registered
	providers []Provider

	// Used to migrate database models.
	models []any

	// Server configuration
	config Config
}

/*
New creates a minimal Server instance.
This is a 'minimal' example of how to configure the library:

	//go:embed assets
	var embeddedFiles embed.FS

	func embedFS(fs embed.FS) http.FileSystem {
		return http.FS(fs)
	}

	type BaseRoutes struct{}

	func (br *BaseRoutes) Register(e *echo.Echo) {
		e.GET("/", func(c echo.Context) error {
			return c.Render(200, "assets/index.html", nil)
		})
	}

	func main() {
		config := server.DevelopmentConfig
		_ = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=5432 sslmode=disable TimeZone=Europe/Oslo",
			"localhost",
			"postgres",
			"postgres",
			"tmp",
		)
		config.DSN = dsn

		s := server.New(config, []provider.RouteProvider{
			&BaseRoutes{},
		}, []any{})

		s.Init(embedFS(embeddedFiles))
	}
*/
func New(config Config, routes []Provider, models []any) Server {
	return Server{
		e: echo.New(),

		providers: routes,
		models:    models,
		config:    config,
	}
}

func (s *Server) Init(fs http.FileSystem) {
	Logging(s.e, s.config.FriendlyLogging)

	if s.config.DSN != "" {
		_, err := database.InitDatabase(s.config.DSN)
		if err != nil {
			log.Fatal("Unable to connect to database: ", err)
			return
		}

		// Migrate all the models
		for _, model := range s.models {
			database.AutoMigrate(model)
		}
	} else {
		log.Info("Skipping database setup, no DSN specified")
	}

	AddMiddlewares(s.e)
	s.registerRoutes()

	// Sets the Jet renderer up.
	s.e.Renderer = renderer.NewTemplateRenderer("www", fs)

	address := fmt.Sprintf(":%d", s.config.HttpPort)
	server.Start(s.e, address, s.config.AutoTLS, s.config.CertKeyPath, s.config.CertPrivateKeyPath, s.config.Domains)
}

func (s *Server) Echo() *echo.Echo {
	return s.e
}

func (s *Server) registerRoutes() {
	for _, provider := range s.providers {
		provider.Register(s.e)
	}
}

func AddMiddlewares(e *echo.Echo) {
	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("image/svg+xml", svg.Minify)
	m.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	m.AddFuncRegexp(regexp.MustCompile("[/+]json$"), json.Minify)
	m.AddFuncRegexp(regexp.MustCompile("[/+]xml$"), xml.Minify)

	// Panics shouldn't kill the server.
	e.Use(middleware.Recover())

	// XSS; etc
	e.Use(middleware.Secure())
}

func Logging(e *echo.Echo, friendly bool) {
	// Whether we will use the easily readable format, or format using common JSON.
	if friendly {
		if l, ok := e.Logger.(*log.Logger); ok {
			l.SetHeader(friendlyHeader)
		}
		log.SetHeader(friendlyHeader)

		e.HideBanner = true

		e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
			Format: requestHeader,
		}))
	} else {
		e.HideBanner = true

		e.Use(middleware.Logger())
	}
}
