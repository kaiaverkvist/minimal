package res

import (
	"github.com/labstack/echo/v4"
	"net/http"
)

type BaseResponse struct {
	Success bool
	Message string
}

type ModelResponse[T any] struct {
	BaseResponse
	Data T
}

func resModel[T any](success bool, model T, message error) ModelResponse[T] {

	msg := ""
	if message != nil {
		msg = message.Error()
	}
	return ModelResponse[T]{
		BaseResponse: BaseResponse{
			Success: success,
			Message: msg,
		},
		Data: model,
	}
}

func Ok[T any](c echo.Context, model T) error {
	return c.JSON(http.StatusOK, resModel(true, model, nil))
}

func OkCode[T any](c echo.Context, code int, model T) error {
	return c.JSON(code, resModel(true, model, nil))
}

func FailCode(c echo.Context, code int, message error) error {
	return c.JSON(code, resModel[any](false, nil, message))
}

func Fail(c echo.Context, message error) error {
	return c.JSON(http.StatusInternalServerError, resModel[any](false, nil, message))
}
