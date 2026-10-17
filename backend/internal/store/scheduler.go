package store

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

func (s Scheduler) CreateExam(ctx context.Context, actorID, courseID, sessionID int64) (int64, error) {
	var id int64
	err := s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "create", Entity: "exams", Invalidate: true}
		if err := tx.QueryRow(ctx, `INSERT INTO exams (course_id, session_id) VALUES ($1, $2) RETURNING id`, courseID, sessionID).Scan(&id); err != nil {
			return value, err
		}
		value.ID = id
		if err := allocateRooms(ctx, tx, id, courseID, sessionID); err != nil {
			return value, err
		}
		var err error
		value.New, err = document(ctx, tx, "exams", id)
		return value, err
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s Scheduler) UpdateExam(ctx context.Context, actorID, id, courseID, sessionID int64) error {
	return s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "update", Entity: "exams", ID: id, Invalidate: true}
		var err error
		value.Old, err = document(ctx, tx, "exams", id)
		if err != nil {
			return value, err
		}
		if _, err := tx.Exec(ctx, `UPDATE exams SET course_id = $2, session_id = $3 WHERE id = $1`, id, courseID, sessionID); err != nil {
			return value, err
		}
		if err := allocateRooms(ctx, tx, id, courseID, sessionID); err != nil {
			return value, err
		}
		value.New, err = document(ctx, tx, "exams", id)
		return value, err
	})
}

func allocateRooms(ctx context.Context, tx pgx.Tx, id, courseID, sessionID int64) error {
	state, err := loadSchedule(ctx, tx)
	if err != nil {
		return err
	}
	enrollment := 0
	for _, course := range state.Courses {
		if course.ID == courseID {
			enrollment = course.Enrollment
		}
	}
	busy := map[int64]bool{}
	for _, allocation := range state.Allocations {
		for _, exam := range state.Exams {
			if allocation.ExamID == exam.ID && exam.ID != id && exam.SessionID == sessionID {
				busy[allocation.ClassroomID] = true
			}
		}
	}
	var available []roomState
	for _, room := range state.Rooms {
		if !busy[room.ID] {
			available = append(available, room)
		}
	}
	rooms, err := chooseRooms(enrollment, available)
	if err != nil {
		return err
	}
	// Keep unchanged allocations and their duties; removed rooms cascade only their own assignments.
	if _, err := tx.Exec(ctx, `DELETE FROM exam_rooms WHERE exam_id = $1 AND NOT (classroom_id = ANY($2::bigint[]))`, id, rooms); err != nil {
		return err
	}
	for _, roomID := range rooms {
		if _, err := tx.Exec(ctx, `INSERT INTO exam_rooms (exam_id, classroom_id) VALUES ($1, $2) ON CONFLICT (exam_id, classroom_id) DO NOTHING`, id, roomID); err != nil {
			return err
		}
	}
	return nil
}

func (s Scheduler) DeleteExam(ctx context.Context, actorID, id int64) error {
	return s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "delete", Entity: "exams", ID: id, Invalidate: true}
		var err error
		value.Old, err = document(ctx, tx, "exams", id)
		if err != nil {
			return value, err
		}
		_, err = tx.Exec(ctx, `DELETE FROM exams WHERE id = $1`, id)
		return value, err
	})
}

func (s Scheduler) CreateAssignment(ctx context.Context, actorID, allocationID, personnelID int64) error {
	return s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "create", Entity: "invigilator_assignments", ID: allocationID, Invalidate: true}
		if _, err := tx.Exec(ctx, `INSERT INTO invigilator_assignments (id, personnel_id) VALUES ($1, $2)`, allocationID, personnelID); err != nil {
			return value, err
		}
		var err error
		value.New, err = document(ctx, tx, value.Entity, allocationID)
		return value, err
	})
}

func (s Scheduler) UpdateAssignment(ctx context.Context, actorID, allocationID, personnelID int64) error {
	return s.update(ctx, actorID, "invigilator_assignments", allocationID, `UPDATE invigilator_assignments SET personnel_id = $2 WHERE id = $1`, allocationID, personnelID)
}

func (s Scheduler) DeleteAssignment(ctx context.Context, actorID, allocationID int64) error {
	return s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "delete", Entity: "invigilator_assignments", ID: allocationID, Invalidate: true}
		var err error
		value.Old, err = document(ctx, tx, value.Entity, allocationID)
		if err != nil {
			return value, err
		}
		_, err = tx.Exec(ctx, `DELETE FROM invigilator_assignments WHERE id = $1`, allocationID)
		return value, err
	})
}

type CourseInput struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Enrollment   int    `json:"enrollment"`
	Term         int    `json:"term"`
	DepartmentID int64  `json:"department_id"`
}

func (s Scheduler) UpdateCourse(ctx context.Context, actorID, id int64, value CourseInput) error {
	return s.update(ctx, actorID, "courses", id, `UPDATE courses SET code = $2, name = $3, kind = $4, enrollment = $5, term = $6, department_id = $7 WHERE id = $1`, id, value.Code, value.Name, value.Kind, value.Enrollment, value.Term, value.DepartmentID)
}

type SessionInput struct {
	Date        string `json:"date"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Description string `json:"description"`
}

func (s Scheduler) UpdateSession(ctx context.Context, actorID, id int64, value SessionInput) error {
	return s.update(ctx, actorID, "sessions", id, `UPDATE sessions SET date = $2, start_time = $3, end_time = $4, description = $5 WHERE id = $1`, id, value.Date, value.StartTime, value.EndTime, value.Description)
}

type ClassroomInput struct {
	Name     string `json:"name"`
	Capacity int    `json:"capacity"`
	Type     string `json:"type"`
	Floor    int    `json:"floor"`
	Active   bool   `json:"active"`
}

func (s Scheduler) UpdateClassroom(ctx context.Context, actorID, id int64, value ClassroomInput) error {
	return s.update(ctx, actorID, "classrooms", id, `UPDATE classrooms SET name = $2, capacity = $3, type = $4, floor = $5, active = $6 WHERE id = $1`, id, value.Name, value.Capacity, value.Type, value.Floor, value.Active)
}

type PersonnelInput struct {
	Title        string `json:"title"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	DepartmentID int64  `json:"department_id"`
}

func (s Scheduler) UpdatePersonnel(ctx context.Context, actorID, id int64, value PersonnelInput) error {
	return s.update(ctx, actorID, "personnel", id, `UPDATE personnel SET title = $2, first_name = $3, last_name = $4, department_id = $5 WHERE id = $1`, id, value.Title, value.FirstName, value.LastName, value.DepartmentID)
}

func (s Scheduler) DecideRequest(ctx context.Context, actorID, id int64, approved bool) error {
	return s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "decide", Entity: "unavailability_requests", ID: id}
		var err error
		value.Old, err = document(ctx, tx, value.Entity, id)
		if err != nil {
			return value, err
		}
		var previous struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(value.Old, &previous); err != nil {
			return value, err
		}
		if previous.Status != "pending" {
			return value, conflict("request_decided", "Only a pending request may be decided.")
		}
		status := "rejected"
		if approved {
			status = "approved"
		}
		if _, err := tx.Exec(ctx, `UPDATE unavailability_requests SET status = $2, decided_by = $3, decided_at = now() WHERE id = $1`, id, status, actorID); err != nil {
			return value, err
		}
		value.New, err = document(ctx, tx, value.Entity, id)
		return value, err
	})
}

func (s Scheduler) SetPublication(ctx context.Context, actorID int64, published bool) error {
	return s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "unpublish", Entity: "schedule_state", ID: 1}
		if published {
			value.Action = "publish"
		}
		var err error
		value.Old, err = document(ctx, tx, value.Entity, 1)
		if err != nil {
			return value, err
		}
		if _, err := tx.Exec(ctx, `UPDATE schedule_state SET published = $1, updated_at = now() WHERE id = 1`, published); err != nil {
			return value, err
		}
		value.New, err = document(ctx, tx, value.Entity, 1)
		return value, err
	})
}
