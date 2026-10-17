package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/reekta92/wad_project/backend/internal/store"
)

func requireJSON(c *fiber.Ctx) error {
	c.Vary("Accept")
	header := c.Get("Accept")
	if header == "" {
		return nil
	}
	quality, specificity := 0.0, -1
	for _, entry := range strings.Split(header, ",") {
		media, params, err := mime.ParseMediaType(strings.TrimSpace(entry))
		if err != nil {
			continue
		}
		level := -1
		switch media {
		case "*/*":
			level = 0
		case "application/*":
			level = 1
		case "application/json":
			level = 2
		}
		if level < 0 || level <= specificity {
			continue
		}
		q := 1.0
		if value, ok := params["q"]; ok {
			q, err = strconv.ParseFloat(value, 64)
			if err != nil || !(q >= 0 && q <= 1) {
				continue
			}
		}
		quality, specificity = q, level
	}
	if quality <= 0 {
		return fiber.NewError(406, "This endpoint currently supports application/json.")
	}
	return nil
}

func decodeInput(c *fiber.Ctx, target any, required ...string) error {
	media, params, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || media != "application/json" || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) {
		return fiber.NewError(415, "Send an application/json request encoded as UTF-8.")
	}
	if !utf8.Valid(c.Body()) {
		return fiber.NewError(400, "The request must contain valid UTF-8 JSON.")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &fields); err != nil || fields == nil {
		return fiber.NewError(400, "Send one JSON object.")
	}
	missing := validationError{}
	for _, name := range required {
		value, present := fields[name]
		if !present || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			missing[name] = "This field is required."
		}
	}
	if len(missing) != 0 {
		return missing
	}
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fiber.NewError(400, "The body contains an unknown field or an invalid field type.")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fiber.NewError(400, "Send one JSON object.")
	}
	return nil
}

func checkInput(value any) error {
	fields := validationError{}
	text := func(name, value string, max int, required bool) {
		if (required && strings.TrimSpace(value) == "") || utf8.RuneCountInString(value) > max {
			fields[name] = "Enter a value within the documented length limit."
		}
	}
	positive := func(name string, value int64) {
		if value < 1 {
			fields[name] = "Enter a positive integer."
		}
	}
	date := func(value string) {
		if _, err := time.Parse(time.DateOnly, value); err != nil {
			fields["date"] = "Enter a valid YYYY-MM-DD date."
		}
	}
	switch v := value.(type) {
	case departmentInput:
		text("name", v.Name, 100, true)
	case store.CourseInput:
		text("code", v.Code, 20, true)
		text("name", v.Name, 100, true)
		positive("department_id", v.DepartmentID)
		positive("enrollment", int64(v.Enrollment))
		if v.Kind != "mandatory" && v.Kind != "elective" {
			fields["kind"] = "Choose mandatory or elective."
		}
		if v.Term < 1 || v.Term > 8 {
			fields["term"] = "Choose a term from 1 to 8."
		}
	case store.ClassroomInput:
		text("name", v.Name, 50, true)
		text("type", v.Type, 50, true)
		positive("capacity", int64(v.Capacity))
	case store.PersonnelInput:
		text("title", v.Title, 100, false)
		text("first_name", v.FirstName, 100, true)
		text("last_name", v.LastName, 100, true)
		positive("department_id", v.DepartmentID)
	case store.SessionInput:
		date(v.Date)
		text("description", v.Description, 500, false)
		start, first := parseTime(v.StartTime)
		end, second := parseTime(v.EndTime)
		if first != nil {
			fields["start_time"] = "Enter a valid HH:MM or HH:MM:SS time."
		}
		if second != nil || (first == nil && !start.Before(end)) {
			fields["end_time"] = "The end time must follow the start time."
		}
	case examInput:
		positive("course_id", v.CourseID)
		positive("session_id", v.SessionID)
	case assignmentInput:
		positive("personnel_id", v.PersonnelID)
		positive("id", v.ID)
	case store.RequestInput:
		positive("personnel_id", v.PersonnelID)
		text("reason", v.Reason, 500, true)
		if v.SessionID != nil {
			positive("session_id", *v.SessionID)
		} else {
			date(v.Date)
		}
		if v.Type != "leave" && v.Type != "medical" && v.Type != "advisory" {
			fields["type"] = "Choose leave, medical, or advisory."
		}
	}
	if len(fields) != 0 {
		return fields
	}
	return nil
}

func parseTime(value string) (time.Time, error) {
	if len(value) == 5 {
		return time.Parse("15:04", value)
	}
	return time.Parse("15:04:05", value)
}
