package minimal

import (
	"fmt"
	renderer "github.com/kaiaverkvist/echo-jet-template-renderer"
	"github.com/kaiaverkvist/minimal/database"
	setup2 "github.com/kaiaverkvist/minimal/setup"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"net/http"
)

type Server struct {
	e *echo.Echo

	// Routes registered
	providers []Provider

	// Used to migrate database models.
	models []any

	// Server configuration
	config setup2.Config
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
func New(config setup2.Config, routes []Provider, models []any) Server {
	return Server{
		e: echo.New(),

		providers: routes,
		models:    models,
		config:    config,
	}
}

func (s *Server) Init(fs http.FileSystem) {
	setup2.Logging(s.e, s.config.FriendlyLogging)

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

	setup2.AddMiddlewares(s.e)
	s.registerRoutes()

	// Sets the Jet renderer up.
	s.e.Renderer = renderer.NewTemplateRenderer("www", fs)

	address := fmt.Sprintf(":%d", s.config.HttpPort)
	setup2.Start(s.e, address, s.config.AutoTLS, s.config.CertKeyPath, s.config.CertPrivateKeyPath, s.config.Domains)
}

func (s *Server) Echo() *echo.Echo {
	return s.e
}

func (s *Server) registerRoutes() {
	for _, provider := range s.providers {
		provider.Register(s.e)
	}
}
