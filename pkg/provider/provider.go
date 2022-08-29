package provider

import "github.com/labstack/echo/v4"

type Provider interface {
	Register(e *echo.Echo)
}
