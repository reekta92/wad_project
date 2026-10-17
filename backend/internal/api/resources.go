package api

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/reekta92/wad_project/backend/internal/store"
)

type departmentInput struct {
	Name string `json:"name"`
}
type examInput struct {
	CourseID  int64 `json:"course_id"`
	SessionID int64 `json:"session_id"`
}
type assignmentInput struct {
	ID          int64 `json:"id"`
	PersonnelID int64 `json:"personnel_id"`
}

type resourceHandler struct {
	databases *store.Databases
	resource  string
}

func businessRoutes(app *fiber.App, databases *store.Databases, verify fiber.Handler) {
	for _, resource := range []string{"departments", "courses", "sessions", "classrooms", "personnel", "unavailability-requests", "exams", "invigilator-assignments"} {
		h := resourceHandler{databases, resource}
		base := "/api/v1/" + resource
		app.Get(base, verify, h.permit, h.read)
		app.Get(base+"/:id", verify, h.permit, h.read)
		app.Post(base, verify, h.permit, h.write)
		app.Put(base+"/:id", verify, h.permit, h.write)
		app.Delete(base+"/:id", verify, h.permit, h.write)
	}
}

func (h resourceHandler) permit(c *fiber.Ctx) error {
	value := currentActor(c)
	if h.resource == "unavailability-requests" && value.Invigilator {
		return c.Next()
	}
	return requireAdministrator(c)
}

func (h resourceHandler) requests(c *fiber.Ctx) store.Requests {
	value := currentActor(c)
	if value.Role == "administrator" {
		return store.Requests{Pool: h.databases.Administrator}
	}
	return store.Requests{Pool: h.databases.Invigilator, PersonnelID: value.User.PersonnelID}
}

func (h resourceHandler) get(ctx context.Context, c *fiber.Ctx, id int64) (json.RawMessage, error) {
	if h.resource == "unavailability-requests" {
		return h.requests(c).Get(ctx, id)
	}
	return store.GetResource(ctx, h.databases.Administrator, h.resource, id)
}

func (h resourceHandler) read(c *fiber.Ctx) error {
	if err := requireJSON(c); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.UserContext(), 10*time.Second)
	defer cancel()
	var body json.RawMessage
	var err error
	if value := c.Params("id"); value != "" {
		id, failure := positiveID(value)
		if failure != nil {
			return fiber.NewError(400, failure.Error())
		}
		body, err = h.get(ctx, c, id)
	} else {
		filter, failure := resourceFilter(h.resource, string(c.Request().URI().QueryString()))
		if failure != nil {
			return fiber.NewError(400, failure.Error())
		}
		if h.resource == "unavailability-requests" {
			body, err = h.requests(c).List(ctx, filter)
		} else {
			body, err = store.ListResources(ctx, h.databases.Administrator, h.resource, filter)
		}
	}
	if err != nil {
		return err
	}
	c.Type("json")
	return c.Send(body)
}

func (h resourceHandler) write(c *fiber.Ctx) error {
	id := int64(0)
	if c.Method() != "POST" {
		var err error
		id, err = positiveID(c.Params("id"))
		if err != nil {
			return fiber.NewError(400, err.Error())
		}
	}
	if c.Method() != "DELETE" {
		if err := requireJSON(c); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(c.UserContext(), 10*time.Second)
	defer cancel()
	s := store.Scheduler{Pool: h.databases.Administrator}
	actor := currentActor(c)
	var err error
	if c.Method() == "DELETE" {
		switch h.resource {
		case "exams":
			err = s.DeleteExam(ctx, actor.User.ID, id)
		case "invigilator-assignments":
			err = s.DeleteAssignment(ctx, actor.User.ID, id)
		case "unavailability-requests":
			err = h.requests(c).Delete(ctx, actor.User.ID, id)
		default:
			err = s.DeleteMaster(ctx, actor.User.ID, h.resource, id)
		}
		if err != nil {
			return err
		}
		return c.SendStatus(204)
	}
	create := c.Method() == "POST"
	switch h.resource {
	case "departments":
		var v departmentInput
		if err := decodeInput(c, &v, "name"); err != nil {
			return err
		}
		if err := checkInput(v); err != nil {
			return err
		}
		if create {
			id, err = s.CreateDepartment(ctx, actor.User.ID, v.Name)
		} else {
			err = s.UpdateDepartment(ctx, actor.User.ID, id, v.Name)
		}
	case "courses":
		var v store.CourseInput
		if err := decodeInput(c, &v, "code", "name", "kind", "enrollment", "term", "department_id"); err != nil {
			return err
		}
		if err := checkInput(v); err != nil {
			return err
		}
		if create {
			id, err = s.CreateCourse(ctx, actor.User.ID, v)
		} else {
			err = s.UpdateCourse(ctx, actor.User.ID, id, v)
		}
	case "sessions":
		var v store.SessionInput
		if err := decodeInput(c, &v, "date", "start_time", "end_time", "description"); err != nil {
			return err
		}
		if err := checkInput(v); err != nil {
			return err
		}
		if create {
			id, err = s.CreateSession(ctx, actor.User.ID, v)
		} else {
			err = s.UpdateSession(ctx, actor.User.ID, id, v)
		}
	case "classrooms":
		var v store.ClassroomInput
		if err := decodeInput(c, &v, "name", "capacity", "type", "floor", "active"); err != nil {
			return err
		}
		if err := checkInput(v); err != nil {
			return err
		}
		if create {
			id, err = s.CreateClassroom(ctx, actor.User.ID, v)
		} else {
			err = s.UpdateClassroom(ctx, actor.User.ID, id, v)
		}
	case "personnel":
		var v store.PersonnelInput
		if err := decodeInput(c, &v, "title", "first_name", "last_name", "department_id"); err != nil {
			return err
		}
		if err := checkInput(v); err != nil {
			return err
		}
		if create {
			id, err = s.CreatePersonnel(ctx, actor.User.ID, v)
		} else {
			err = s.UpdatePersonnel(ctx, actor.User.ID, id, v)
		}
	case "exams":
		var v examInput
		if err := decodeInput(c, &v, "course_id", "session_id"); err != nil {
			return err
		}
		if err := checkInput(v); err != nil {
			return err
		}
		if create {
			id, err = s.CreateExam(ctx, actor.User.ID, v.CourseID, v.SessionID)
		} else {
			err = s.UpdateExam(ctx, actor.User.ID, id, v.CourseID, v.SessionID)
		}
	case "invigilator-assignments":
		var v assignmentInput
		if create {
			if err := decodeInput(c, &v, "id", "personnel_id"); err != nil {
				return err
			}
		} else {
			var input struct {
				PersonnelID int64 `json:"personnel_id"`
			}
			if err := decodeInput(c, &input, "personnel_id"); err != nil {
				return err
			}
			v.ID, v.PersonnelID = id, input.PersonnelID
		}
		if err := checkInput(v); err != nil {
			return err
		}
		if create {
			id = v.ID
			err = s.CreateAssignment(ctx, actor.User.ID, id, v.PersonnelID)
		} else {
			err = s.UpdateAssignment(ctx, actor.User.ID, id, v.PersonnelID)
		}
	case "unavailability-requests":
		var v store.RequestInput
		required := []string{"type", "reason"}
		if actor.Role == "administrator" {
			required = append(required, "personnel_id")
		}
		if err := decodeInput(c, &v, required...); err != nil {
			return err
		}
		if actor.Role != "administrator" {
			if v.PersonnelID != 0 && v.PersonnelID != *actor.User.PersonnelID {
				return fiber.NewError(403, "You can create and edit requests only for yourself.")
			}
			v.PersonnelID = *actor.User.PersonnelID
		}
		if err := checkInput(v); err != nil {
			return err
		}
		if create {
			id, err = h.requests(c).Create(ctx, actor.User.ID, v)
		} else {
			err = h.requests(c).Update(ctx, actor.User.ID, id, v)
		}
	}
	if err != nil {
		return err
	}
	body, err := h.get(ctx, c, id)
	if err != nil {
		return err
	}
	if create {
		c.Set("Location", "/api/v1/"+h.resource+"/"+strconv.FormatInt(id, 10))
		c.Status(201)
	}
	c.Type("json")
	return c.Send(body)
}
