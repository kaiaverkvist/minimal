# Minimal
    Minimal is a small, opinionated wrapper around Echo and Gorm.
    Postgres set up and ready to go.

# Absolute minimal configuration for a development environment
```go
//go:embed assets
var embeddedFiles embed.FS

func embedFS(fs embed.FS) http.FileSystem {
	return http.FS(fs)
}

func main() {
	config := minimal.DevelopmentConfig
	dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=5432 sslmode=disable TimeZone=Europe/Oslo",
		"localhost",
		"postgres",
		"postgres",
		"tmp",
	)
	config.DSN = dsn

	s := minimal.New(config, []minimal.Provider{
		&BaseRoutes{},
	}, []any{})

	s.Init(embedFS(embeddedFiles))
}
```

Routes defined like this:
```go
type BaseRoutes struct{}

func (br *BaseRoutes) Register(e *echo.Echo) {
	e.GET("/", func(c echo.Context) error {
		return c.Render(200, "assets/index.html", nil)
	})
}
```

## Res package
Instead of using `c.JSON`, you can use the `res` package which wraps your data type in a general success and failure struct.

## Auto-generated API Resource
The snippet below will set up
````go
type UpdateTest struct {
	Name        string
	Description string
}

type Test struct {
	repository.BaseModel

	Name        string
	Description string
}

type TestResource struct {
	minimal.Resource[Test]
}

func NewTestResource() *TestResource {
	api := TestResource{
		minimal.Resource[Test]{
			Name: "tests",
		},
	}

	api.SetWriteBindType(&UpdateTest{})
	api.SetCreateBindType(&UpdateTest{})

	api.CanDeleteById(func(c echo.Context) bool {
		return false
	})

	return &api
}
````