package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/reekta92/wad_project/backend/internal/store"
)

func workflowRoutes(app *fiber.App, databases *store.Databases, verify fiber.Handler) {
	h := workflowHandler{databases}
	for _, path := range []string{"dashboard", "reports/workload", "reports/availability", "me/duties", "me/workload"} {
		permission := requireAdministrator
		if path == "me/duties" || path == "me/workload" {
			permission = requireInvigilator
		}
		app.Get("/api/v1/"+path, verify, permission, h.read(path))
	}
	audit := resourceHandler{databases, "audit-logs"}
	app.Get("/api/v1/audit-logs", verify, requireAdministrator, audit.read)
	app.Get("/api/v1/audit-logs/:id", verify, requireAdministrator, audit.read)
	app.Post("/api/v1/schedule/automatic-planning", verify, requireAdministrator, h.write("plan"))
	app.Post("/api/v1/invigilator-assignments/automatic", verify, requireAdministrator, h.write("staff"))
	app.Post("/api/v1/unavailability-requests/:id/decision", verify, requireAdministrator, h.write("decide"))
	app.Put("/api/v1/schedule/publication", verify, requireAdministrator, h.write("publish"))
	app.Delete("/api/v1/schedule", verify, requireAdministrator, h.write("clear"))
}

func requireInvigilator(c *fiber.Ctx) error {
	value := currentActor(c)
	if !value.Invigilator || value.User.PersonnelID == nil {
		return fiber.NewError(403, "A linked invigilator account is required.")
	}
	return c.Next()
}

type workflowHandler struct{ databases *store.Databases }

func (h workflowHandler) read(path string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := representation(c, path == "reports/workload" || path == "reports/availability" || path == "me/duties"); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(c.UserContext(), 10*time.Second)
		defer cancel()
		var body json.RawMessage
		var err error
		if path == "dashboard" {
			if len(c.Request().URI().QueryString()) != 0 {
				return fiber.NewError(400, "The dashboard does not accept filters.")
			}
			body, err = h.databases.Dashboard(ctx)
		} else {
			filter, failure := resourceFilter(path, string(c.Request().URI().QueryString()))
			if failure != nil {
				return fiber.NewError(400, failure.Error())
			}
			switch path {
			case "reports/workload":
				body, err = h.databases.Workload(ctx, filter)
			case "reports/availability":
				body, err = h.databases.Availability(ctx, filter)
			case "me/duties":
				body, err = h.databases.OwnDuties(ctx, *currentActor(c).User.PersonnelID, filter)
			case "me/workload":
				if c.Query("page") != "" || c.Query("page_size") != "" {
					return fiber.NewError(400, "The workload summary accepts only date filters.")
				}
				body, err = h.databases.OwnWorkload(ctx, *currentActor(c).User.PersonnelID, filter)
			}
		}
		if err != nil {
			return err
		}
		return sendRepresentation(c, body)
	}
}

func (h workflowHandler) write(action string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if action != "clear" {
			if err := representation(c, false); err != nil {
				return err
			}
		}
		ctx, cancel := context.WithTimeout(c.UserContext(), 10*time.Second)
		defer cancel()
		s := store.Scheduler{Pool: h.databases.Administrator}
		actorID := currentActor(c).User.ID
		switch action {
		case "clear":
			if err := s.Clear(ctx, actorID); err != nil {
				return err
			}
			return c.SendStatus(204)
		case "plan":
			var input struct {
				Season string `json:"season"`
			}
			if err := decodeInput(c, &input); err != nil {
				return err
			}
			if input.Season == "" {
				input.Season = "all"
			}
			if input.Season != "all" && input.Season != "autumn" && input.Season != "spring" {
				return validationError{"season": "Choose all, autumn, or spring."}
			}
			result, err := s.Plan(ctx, actorID, input.Season)
			if err != nil {
				return err
			}
			return sendValue(c, result)
		case "staff":
			var input struct {
				Scope        string `json:"scope"`
				AllocationID int64  `json:"allocation_id"`
			}
			if err := decodeInput(c, &input, "scope"); err != nil {
				return err
			}
			if input.Scope != "all" && input.Scope != "one" {
				return validationError{"scope": "Choose all or one."}
			}
			if input.Scope == "one" && input.AllocationID < 1 || input.Scope == "all" && input.AllocationID != 0 {
				return validationError{"allocation_id": "Supply a positive allocation ID only for scope one."}
			}
			result, err := s.Staff(ctx, actorID, input.Scope, input.AllocationID)
			if err != nil {
				return err
			}
			return sendValue(c, result)
		case "decide":
			id, err := positiveID(c.Params("id"))
			if err != nil {
				return fiber.NewError(400, err.Error())
			}
			var input struct {
				Approved bool `json:"approved"`
			}
			if err := decodeInput(c, &input, "approved"); err != nil {
				return err
			}
			if err := s.DecideRequest(ctx, actorID, id, input.Approved); err != nil {
				return err
			}
			body, err := store.GetResource(ctx, h.databases.Administrator, "unavailability-requests", id)
			if err != nil {
				return err
			}
			return sendRepresentation(c, body)
		case "publish":
			var input struct {
				Published bool `json:"published"`
			}
			if err := decodeInput(c, &input, "published"); err != nil {
				return err
			}
			if err := s.SetPublication(ctx, actorID, input.Published); err != nil {
				return err
			}
			body, err := store.GetResource(ctx, h.databases.Administrator, "schedule/publication", 1)
			if err != nil {
				return err
			}
			return sendRepresentation(c, body)
		}
		return fiber.ErrNotFound
	}
}
