package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/reekta92/wad_project/backend/internal/store"
)

var resourceFilters = map[string][]string{
	"departments":             {"name"},
	"courses":                 {"department_id", "term", "kind", "q"},
	"sessions":                {"date_from", "date_to"},
	"classrooms":              {"active", "floor", "min_capacity"},
	"personnel":               {"department_id", "q"},
	"unavailability-requests": {"personnel_id", "date_from", "date_to", "status", "type"},
	"exams":                   {"course_id", "department_id", "term", "session_id", "date_from", "date_to"},
	"invigilator-assignments": {"personnel_id", "exam_id", "session_id"},
}

func positiveID(value string) (int64, error) {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("IDs must be positive integers.")
	}
	return n, nil
}

func resourceFilter(resource, raw string) (store.ResourceFilter, error) {
	f := store.ResourceFilter{Page: 1, PageSize: 20}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return f, fmt.Errorf("Invalid query parameters.")
	}
	allowed := map[string]bool{"page": true, "page_size": true}
	for _, key := range resourceFilters[resource] {
		allowed[key] = true
	}
	for key, entries := range values {
		if !allowed[key] || len(entries) != 1 || strings.TrimSpace(entries[0]) == "" {
			return f, fmt.Errorf("Invalid or repeated query parameter: %s.", key)
		}
	}
	for name, target := range map[string]*int{"page": &f.Page, "page_size": &f.PageSize, "term": &f.Term, "min_capacity": &f.MinCapacity} {
		if value := values.Get(name); value != "" {
			n, err := strconv.ParseInt(value, 10, 32)
			if err != nil || n < 1 {
				return f, fmt.Errorf("%s must be a positive integer.", name)
			}
			*target = int(n)
		}
	}
	if f.PageSize > 100 || f.Term > 8 {
		return f, fmt.Errorf("page_size must not exceed 100; term must be between 1 and 8.")
	}
	for name, target := range map[string]*int64{"department_id": &f.DepartmentID, "course_id": &f.CourseID, "session_id": &f.SessionID, "personnel_id": &f.PersonnelID, "exam_id": &f.ExamID} {
		if value := values.Get(name); value != "" {
			*target, err = positiveID(value)
			if err != nil {
				return f, err
			}
		}
	}
	if value := values.Get("floor"); value != "" {
		n, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return f, fmt.Errorf("floor must be an integer.")
		}
		floor := int(n)
		f.Floor = &floor
	}
	if value := values.Get("active"); value != "" {
		if value != "true" && value != "false" {
			return f, fmt.Errorf("active must be true or false.")
		}
		active := value == "true"
		f.Active = &active
	}
	f.DateFrom, f.DateTo = values.Get("date_from"), values.Get("date_to")
	for _, date := range []string{f.DateFrom, f.DateTo} {
		if date != "" {
			if _, err := time.Parse(time.DateOnly, date); err != nil {
				return f, fmt.Errorf("Dates must use valid YYYY-MM-DD values.")
			}
		}
	}
	if f.DateFrom != "" && f.DateTo != "" && f.DateFrom > f.DateTo {
		return f, fmt.Errorf("date_from must not follow date_to.")
	}
	f.Query, f.Name = strings.TrimSpace(values.Get("q")), strings.TrimSpace(values.Get("name"))
	if !validSearchText(f.Query) || !validSearchText(f.Name) {
		return f, fmt.Errorf("Search filters must contain valid UTF-8 text of at most 100 bytes, without NUL characters.")
	}
	f.Kind, f.Status, f.Type = values.Get("kind"), values.Get("status"), values.Get("type")
	if f.Kind != "" && f.Kind != "mandatory" && f.Kind != "elective" {
		return f, fmt.Errorf("Invalid course kind.")
	}
	if f.Status != "" && f.Status != "pending" && f.Status != "approved" && f.Status != "rejected" {
		return f, fmt.Errorf("Invalid request status.")
	}
	if f.Type != "" && f.Type != "leave" && f.Type != "medical" && f.Type != "advisory" {
		return f, fmt.Errorf("Invalid request type.")
	}
	return f, nil
}
