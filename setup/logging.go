package setup

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

const (
	friendlyHeader = "⇨ ${time_rfc3339} (${short_file}:${line}) ${level}  "
	requestHeader  = "⇨ ${time_rfc3339} HTTP  ${method} ${uri} -> RESP ${status} (took ${latency_human}) (▼${bytes_in}B  ▲${bytes_out}B)\n"
)

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
