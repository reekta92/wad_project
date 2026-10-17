package api

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/reekta92/wad_project/backend/internal/store"
)

type validationError map[string]string

func (e validationError) Error() string { return "Check the highlighted fields." }

func respondError(c *fiber.Ctx, err error) error {
	status, code, message := 500, "Internal Server Error", "An internal error occurred."
	var fields validationError
	var expected *fiber.Error
	var rule *store.RuleError
	var database *pgconn.PgError
	switch {
	case errors.As(err, &fields):
		status, code, message = 400, "validation", fields.Error()
	case errors.As(err, &expected):
		status, code, message = expected.Code, http.StatusText(expected.Code), expected.Message
	case errors.Is(err, store.ErrNotFound):
		status, code, message = 404, "not_found", "This resource could not be found."
	case errors.Is(err, store.ErrInactiveActor):
		status, code, message = 403, "inactive_account", "Your account is no longer enabled for this action."
	case errors.As(err, &rule):
		status, code, message = 409, rule.Code, rule.Message
	case errors.As(err, &database):
		switch database.Code {
		case "23505":
			status, code, message = 409, "duplicate", "A resource with these unique values already exists."
		case "23503":
			status, code, message = 409, "reference_conflict", "A referenced resource is missing or this resource is still in use."
		case "23514", "23502", "22007", "22008", "22003":
			status, code, message = 400, "invalid_value", "One or more values do not meet the resource requirements."
		}
	}
	body := fiber.Map{"code": code, "message": message, "request_id": c.GetRespHeader("X-Request-ID")}
	if fields != nil {
		body["fields"] = fields
	}
	return c.Status(status).JSON(fiber.Map{"error": body})
}
