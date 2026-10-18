package api

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/reekta92/wad_project/backend/internal/backup"
	"github.com/reekta92/wad_project/backend/internal/config"
	"github.com/reekta92/wad_project/backend/internal/store"
	"github.com/reekta92/wad_project/backend/internal/xlsx"
)

func downloadRoutes(app *fiber.App, cfg config.Config, databases *store.Databases, verify fiber.Handler) {
	service := backup.New(cfg.AdminURL)
	app.Get("/api/v1/schedule/export", verify, func(c *fiber.Ctx) error {
		if err := chooseRepresentation(c, []string{xlsx.MediaType}); err != nil {
			return err
		}
		filter, err := scheduleFilter(string(c.Request().URI().QueryString()))
		if err != nil {
			return fiber.NewError(400, err.Error())
		}
		if c.Query("page") != "" || c.Query("page_size") != "" {
			return fiber.NewError(400, "Exports accept schedule filters, without page or page_size.")
		}
		ctx, cancel := context.WithTimeout(c.UserContext(), 10*time.Second)
		defer cancel()
		current := currentActor(c).Role == "administrator"
		pool := databases.Reader
		if current {
			pool = databases.Administrator
		}
		rows, err := store.ExportSchedule(ctx, pool, filter, current)
		if err != nil {
			return downloadError(err)
		}
		body, err := xlsx.Schedule(rows)
		if err != nil {
			return downloadError(err)
		}
		c.Set("Content-Type", xlsx.MediaType)
		c.Set("Content-Disposition", `attachment; filename="exam-schedule.xlsx"`)
		return c.Send(body)
	})
	app.Post("/api/v1/backups", verify, requireAdministrator, func(c *fiber.Ctx) error {
		if err := chooseRepresentation(c, []string{backup.MediaType}); err != nil {
			return err
		}
		if len(c.Body()) != 0 || len(c.Request().URI().QueryString()) != 0 {
			return fiber.NewError(400, "Backups do not accept a body, query, or destination path.")
		}
		ctx, cancel := context.WithTimeout(c.UserContext(), 45*time.Second)
		defer cancel()
		archive, err := service.Create(ctx)
		if errors.Is(err, backup.ErrBusy) {
			c.Set("Retry-After", "5")
			return fiber.NewError(429, "A backup download is already in progress. Try again shortly.")
		}
		if err != nil {
			return fiber.NewError(503, "The backup could not be completed. Check the server's PostgreSQL client and database configuration.")
		}
		stat, err := archive.Stat()
		if err != nil {
			archive.Close()
			return err
		}
		c.Set("Content-Type", backup.MediaType)
		c.Set("Content-Disposition", `attachment; filename="application.dump"`)
		// Fiber closes this stream after delivery or disconnect, releasing the slot and temporary files.
		return c.SendStream(archive, int(stat.Size()))
	})
}

func downloadError(err error) error {
	if errors.Is(err, store.ErrExportLimit) {
		return fiber.NewError(413, "The export exceeds 10,000 exams, 32 MiB of source data, or a spreadsheet cell limit. Narrow the filters.")
	}
	return err
}
