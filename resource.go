package minimal

import (
	"errors"
	patch "github.com/geraldo-labs/merge-struct"
	"github.com/kaiaverkvist/minimal/database"
	"github.com/kaiaverkvist/minimal/res"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"gorm.io/gorm"
	"net/http"
	"reflect"
	"strconv"
)

var (
	ErrorNoResourceAccess = errors.New("no resource access")
	ErrorNoResourceFound  = errors.New("no resource found")
	ErrorDatabase         = errors.New("database problem")
	ErrorNoBindType       = errors.New("unable to handle this request")
	ErrorInvalidData      = errors.New("bad data")
	ErrorInvalidID        = errors.New("bad id")
)

// Resource is an automatic REST api module which lets the consumer simply define the resource and it will have
// associated database code, et.c. automatically set up.
type Resource[T any] struct {
	Name string

	// Hooking into registration, by consumer.
	onRegister func(e *echo.Echo)

	// List ALL operation.
	canListAll   func(c echo.Context) bool
	listAllQuery func(c echo.Context, q *gorm.DB) ([]T, error)

	// List by ID operation.
	canListById   func(c echo.Context, entity T) bool
	listByIdQuery func(c echo.Context, q *gorm.DB, id uint) (*T, error)

	// Write by ID operation.
	canWriteById   func(c echo.Context, entity T) bool
	writeBindType  any
	writeByIdQuery func(c echo.Context, q *gorm.DB, id uint, new any) error

	// Create operation.
	canCreate      func(c echo.Context) bool
	createBindType any

	// Used in case patching is not sufficient for creation of the entity
	createTransformer func(c echo.Context) (*T, error)

	// Delete by ID operation.
	canDeleteById   func(c echo.Context, entity T) bool
	deleteByIdQuery func(c echo.Context, q *gorm.DB, entity T) error

	middlewares []echo.MiddlewareFunc
}

// Register is called when minimal initializes, and will add routes and trigger the automigration.
func (r *Resource[T]) Register(e *echo.Echo) {
	// Consumer can hook into registration by overriding.
	if r.onRegister != nil {
		r.onRegister(e)
	}

	if r.listAllQuery == nil {
		// Default querying function for list all.
		r.listAllQuery = func(c echo.Context, q *gorm.DB) ([]T, error) {
			var result []T
			tx := q.Find(&result)

			if tx.Error != nil {
				return nil, ErrorNoResourceFound
			}

			return result, nil
		}
	}

	if r.listByIdQuery == nil {
		// Default for list by id
		r.listByIdQuery = func(c echo.Context, q *gorm.DB, id uint) (*T, error) {
			var result T
			tx := q.First(&result, "id = ?", id)

			if r.canListById != nil {
				if !r.canListById(c, result) {
					return nil, ErrorNoResourceAccess
				}
			}

			if tx.Error != nil {
				if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
					return nil, ErrorNoResourceFound
				}

				return nil, tx.Error
			}

			return &result, nil
		}
	}

	if r.writeByIdQuery == nil {
		r.writeByIdQuery = func(c echo.Context, q *gorm.DB, id uint, new any) error {
			var result T
			tx := q.First(&result, "id = ?", id)

			if r.canWriteById != nil {
				if !r.canWriteById(c, result) {
					return ErrorNoResourceAccess
				}
			}

			_, err := patch.Struct(&result, new)
			if err != nil {
				log.Error("Patching failed: ", err)
				return ErrorInvalidData
			}

			tx2 := database.Db.Save(result)
			if tx2.Error != nil {
				return tx2.Error
			}

			if tx.Error != nil {
				return tx.Error
			}

			return nil
		}
	}

	if r.deleteByIdQuery == nil {
		r.deleteByIdQuery = func(c echo.Context, q *gorm.DB, entity T) error {
			tx := database.Db.Delete(&entity)

			if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
				return ErrorNoResourceFound
			}

			if tx.Error != nil {
				return tx.Error
			}

			return nil
		}
	}

	if database.Db != nil {
		log.Info("Initialized resource: ", r.Name)
		database.AutoMigrate(new(T))
	} else {
		log.Info("Uninitialized database, skipping..")
	}

	group := e.Group(r.Name)
	group.GET("", r.getAll, r.middlewares...)
	group.GET("/:id", r.getById, r.middlewares...)
	group.PUT("/:id", r.writeById, r.middlewares...)
	group.POST("", r.create, r.middlewares...)
	group.DELETE("/:id", r.deleteById, r.middlewares...)
}

func (r *Resource[T]) getAll(c echo.Context) error {
	// Access control check
	if r.canListAll != nil {
		if !r.canListAll(c) {
			return res.FailCode(c, http.StatusForbidden, ErrorNoResourceAccess)
		}
	}

	m, err := r.listAllQuery(c, database.Db)
	if err != nil {
		if errors.Is(err, ErrorNoResourceFound) {
			return res.FailCode(c, http.StatusNotFound, err)
		}

		log.Errorf("Could not list all for resource %s: %s", reflect.TypeOf(r), err)
		return res.FailCode(c, http.StatusInternalServerError, ErrorDatabase)
	}

	return res.Ok(c, m)
}

func (r *Resource[T]) getById(c echo.Context) error {
	// Parse the ID parameter, or fail.
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return res.FailCode(c, http.StatusBadRequest, ErrorInvalidID)
	}

	m, err := r.listByIdQuery(c, database.Db, uint(id))
	if err != nil {
		if errors.Is(err, ErrorNoResourceFound) {
			return res.FailCode(c, http.StatusNotFound, ErrorNoResourceFound)
		}

		// When we don't have access to the resource.
		if errors.Is(err, ErrorNoResourceAccess) {
			return res.FailCode(c, http.StatusForbidden, ErrorNoResourceAccess)
		}

		log.Errorf("Could not get by id for resource %s: %s", reflect.TypeOf(r), err)
		return res.FailCode(c, http.StatusInternalServerError, ErrorDatabase)
	}

	return res.Ok(c, m)
}

func (r *Resource[T]) writeById(c echo.Context) error {
	// Check that we have a bind type set up already. If not, we must fail the call.
	if r.writeBindType == nil {
		log.Error("Cannot write without a bind type set up. Call SetWriteBindType.")
		return res.FailCode(c, http.StatusInternalServerError, ErrorNoBindType)
	}

	// Try to instantiate the "DTO" type, and bind to it.
	boundType := reflect.TypeOf(r.writeBindType)
	boundPtr := reflect.New(boundType)
	bound := boundPtr.Interface()
	if err := c.Bind(bound); err != nil {
		log.Error("Binding failed: ", err)
		return res.FailCode(c, http.StatusBadRequest, ErrorInvalidData)
	}

	// Parse the ID parameter, or fail.
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return res.FailCode(c, http.StatusBadRequest, ErrorInvalidID)
	}

	err = r.writeByIdQuery(c, database.Db, uint(id), bound)
	if err != nil {
		// Tried to write a non existant resource.
		if errors.Is(err, ErrorNoResourceFound) {
			return res.FailCode(c, http.StatusNotFound, ErrorNoResourceFound)
		}

		// When we don't have access to the resource.
		if errors.Is(err, ErrorNoResourceAccess) {
			return res.FailCode(c, http.StatusForbidden, ErrorNoResourceAccess)
		}

		log.Errorf("Could not write by id for resource %s: %s", reflect.TypeOf(r), err)
		return res.FailCode(c, http.StatusInternalServerError, ErrorDatabase)
	}

	return c.NoContent(http.StatusOK)
}

func (r *Resource[T]) create(c echo.Context) error {
	// Check that we can actually create the resource.
	if r.canCreate != nil {
		if !r.canCreate(c) {
			return res.FailCode(c, http.StatusForbidden, ErrorNoResourceAccess)
		}
	}

	// Patch data onto the structure.
	var model T
	if r.createTransformer != nil {
		transformedModel, err := r.createTransformer(c)
		if err != nil {
			return res.FailCode(c, http.StatusBadRequest, err)
		}

		if transformedModel != nil {
			model = *transformedModel
		}
	} else {
		// Check that we have a bind type set up already. If not, we must fail the call.
		if r.createBindType == nil {
			log.Error("Cannot write without a bind type set up. Call SetCreateBindType.")
			return res.FailCode(c, http.StatusInternalServerError, ErrorNoBindType)
		}

		// Try to instantiate the "DTO" type, and bind to it.
		boundType := reflect.TypeOf(r.createBindType)
		boundPtr := reflect.New(boundType)
		bound := boundPtr.Interface()
		if err := c.Bind(bound); err != nil {
			log.Error("Binding failed: ", err)
			return res.FailCode(c, http.StatusBadRequest, ErrorInvalidData)
		}

		_, err := patch.Struct(&model, bound)
		if err != nil {
			log.Error("Patching failed: ", err)
			return res.FailCode(c, http.StatusBadRequest, ErrorInvalidData)
		}
	}

	// Finally create.
	tx := database.Db.Create(&model)
	if tx.Error != nil {
		return res.FailCode(c, http.StatusInternalServerError, ErrorDatabase)
	}

	return c.NoContent(http.StatusOK)
}

func (r *Resource[T]) deleteById(c echo.Context) error {
	// Parse the ID parameter, or fail.
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return res.FailCode(c, http.StatusBadRequest, ErrorInvalidID)
	}

	var result T
	tx := database.Db.First(&result, "id = ?", id)
	if tx.Error != nil {
		err = tx.Error
	}

	if r.canDeleteById != nil {
		if !r.canDeleteById(c, result) {
			return ErrorNoResourceAccess
		}
	}

	err = r.deleteByIdQuery(c, database.Db, result)
	if err != nil {
		// Tried to delete a non existant entity.
		if errors.Is(err, ErrorNoResourceFound) {
			return res.FailCode(c, http.StatusNotFound, ErrorNoResourceFound)
		}

		// When we don't have access to the resource.
		if errors.Is(err, ErrorNoResourceAccess) {
			return res.FailCode(c, http.StatusForbidden, ErrorNoResourceAccess)
		}

		// Otherwise, send them a 500.
		log.Errorf("Could not delete by id for resource %s: %s", reflect.TypeOf(r), err)
		return res.FailCode(c, http.StatusInternalServerError, ErrorDatabase)
	}

	return c.NoContent(http.StatusOK)
}

func (r *Resource[T]) Middlewares(m ...echo.MiddlewareFunc) {
	r.middlewares = m
}

// CanListAll takes a predicate and determines whether the operation can proceed.
func (r *Resource[T]) CanListAll(predicate func(c echo.Context) bool) {
	r.canListAll = predicate
}

// CanListById takes a predicate and determines whether the operation can proceed.
func (r *Resource[T]) CanListById(predicate func(c echo.Context, entity T) bool) {
	r.canListById = predicate
}

// CanWriteById takes a predicate and determines whether the operation can proceed.
func (r *Resource[T]) CanWriteById(predicate func(c echo.Context, entity T) bool) {
	r.canWriteById = predicate
}

// CanDeleteById takes a predicate and determines whether the operation can proceed.
func (r *Resource[T]) CanDeleteById(predicate func(c echo.Context, entity T) bool) {
	r.canDeleteById = predicate
}

// OverrideListAllQuery lets consumers override the query used in the "List All" operation.
func (r *Resource[T]) OverrideListAllQuery(predicate func(c echo.Context, q *gorm.DB) ([]T, error)) {
	r.listAllQuery = predicate
}

// OverrideListByIdQuery lets consumers override the query used in the "List By Id" operation.
func (r *Resource[T]) OverrideListByIdQuery(predicate func(c echo.Context, q *gorm.DB, id uint) (*T, error)) {
	r.listByIdQuery = predicate
}

// OverrideDeleteByIdQuery lets consumers override the query used in the "Delete By Id" operation.
func (r *Resource[T]) OverrideDeleteByIdQuery(predicate func(c echo.Context, q *gorm.DB, entity T) error) {
	r.deleteByIdQuery = predicate
}

// SetWriteBindType will typically be a DTO struct.
func (r *Resource[T]) SetWriteBindType(t any) {
	r.writeBindType = t
}

// SetCreateBindType will typically be a DTO struct.
func (r *Resource[T]) SetCreateBindType(t any) {
	r.createBindType = t
}

func (r *Resource[T]) SetCreateTransformer(tf func(c echo.Context) (*T, error)) {
	r.createTransformer = tf
}

// OnRegister sets the registration hook to argument f.
func (r *Resource[T]) OnRegister(f func(e *echo.Echo)) {
	r.onRegister = f
}
