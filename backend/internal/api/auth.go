package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/reekta92/wad_project/backend/internal/auth"
	"github.com/reekta92/wad_project/backend/internal/store"
)

type actor struct {
	User           store.User
	Role, Username string
	Invigilator    bool
}

func authenticate(databases *store.Databases, verifier *auth.Verifier) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		parts := strings.Fields(c.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.Set("WWW-Authenticate", "Bearer")
			return fiber.NewError(401, "Sign in to access this resource.")
		}
		ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
		defer cancel()
		claims, err := verifier.Verify(ctx, parts[1])
		if err != nil {
			c.Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			return fiber.NewError(401, "Your session is invalid or expired. Sign in again.")
		}
		value := actor{Username: claims.Username, Role: "member", Invigilator: claims.HasRole("invigilator")}
		if claims.HasRole("administrator") {
			if !claims.StrongAuthentication() {
				return fiber.NewError(403, "Administrator access requires a passkey or password plus TOTP. Complete setup, then sign in again.")
			}
			value.Role = "administrator"
		} else if !claims.HasRole("member") {
			return fiber.NewError(403, "Your account has no application role.")
		}
		value.User, err = databases.User(ctx, claims.Issuer, claims.Subject)
		if errors.Is(err, pgx.ErrNoRows) {
			return fiber.NewError(403, "Your account is not enabled for this application.")
		}
		if err != nil {
			return fiber.NewError(503, "Account information is temporarily unavailable.")
		}
		if value.Invigilator && value.User.PersonnelID == nil {
			return fiber.NewError(403, "Your invigilator account is not linked to a personnel record.")
		}
		c.Locals("actor", value)
		return c.Next()
	}
}

func currentActor(c *fiber.Ctx) actor { return c.Locals("actor").(actor) }

func requireAdministrator(c *fiber.Ctx) error {
	if currentActor(c).Role != "administrator" {
		return fiber.NewError(403, "Administrator access is required.")
	}
	return c.Next()
}

func profile(c *fiber.Ctx) error {
	if err := representation(c, false); err != nil {
		return err
	}
	value := currentActor(c)
	permissions := []string{}
	if value.Invigilator {
		permissions = append(permissions, "invigilator")
	}
	return sendValue(c, fiber.Map{"id": value.User.ID, "username": value.Username, "role": value.Role, "permissions": permissions, "personnel_id": value.User.PersonnelID})
}
