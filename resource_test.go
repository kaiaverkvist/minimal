package minimal

import (
	"errors"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
)

type TestData struct {
	Name string
}

type TestResource struct {
	Resource[TestData]
}

func TestNew(t *testing.T) {
	api := TestResource{Resource[TestData]{}}

	assert.NotNil(t, api)
}

func TestResource_OverrideListAllQuery(t *testing.T) {
	api := TestResource{Resource[TestData]{}}

	api.OverrideListAllQuery(func(c echo.Context, q *gorm.DB) ([]TestData, error) {
		return nil, errors.New("tshsaas")
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	b, err := api.listAllQuery(c, &gorm.DB{})
	assert.NotNil(t, err)
	assert.Nil(t, b)

	api.Register(e)

	b, err = api.listAllQuery(c, &gorm.DB{})
	assert.NotNil(t, err)
	assert.Nil(t, b)
}
