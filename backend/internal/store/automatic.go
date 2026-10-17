package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
)

type BatchResult struct {
	Count int     `json:"count"`
	IDs   []int64 `json:"ids"`
}

func scheduleDocument(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
	var body json.RawMessage
	err := tx.QueryRow(ctx, `SELECT jsonb_build_object(
        'exams', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM exams t),
        'rooms', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM exam_rooms t),
        'assignments', (SELECT coalesce(jsonb_agg(to_jsonb(t) ORDER BY id), '[]'::jsonb) FROM invigilator_assignments t))`).Scan(&body)
	return body, err
}

// Plan preserves existing exams and tries course IDs, then chronological sessions, in order.
func (s Scheduler) Plan(ctx context.Context, actorID int64, season string) (BatchResult, error) {
	result := BatchResult{IDs: []int64{}}
	if season != "all" && season != "autumn" && season != "spring" {
		return result, fmt.Errorf("unknown planning season")
	}
	err := s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "automatic_planning", Entity: "exams"}
		var err error
		value.Old, err = scheduleDocument(ctx, tx)
		if err != nil {
			return value, err
		}
		state, err := loadSchedule(ctx, tx)
		if err != nil {
			return value, err
		}
		if err := state.validate(false); err != nil {
			return value, err
		}
		sessions := append([]sessionState(nil), state.Sessions...)
		sort.Slice(sessions, func(i, j int) bool {
			if sessions[i].Date != sessions[j].Date {
				return sessions[i].Date < sessions[j].Date
			}
			if sessions[i].StartTime != sessions[j].StartTime {
				return sessions[i].StartTime < sessions[j].StartTime
			}
			return sessions[i].ID < sessions[j].ID
		})
		planned := map[int64]bool{}
		for _, exam := range state.Exams {
			planned[exam.CourseID] = true
		}
		for _, course := range state.Courses {
			if planned[course.ID] || season == "autumn" && course.Term%2 == 0 || season == "spring" && course.Term%2 != 0 {
				continue
			}
			placed := false
			reason := "No sessions are available."
			for _, session := range sessions {
				if err := ctx.Err(); err != nil {
					return value, err
				}
				rooms, err := state.placement(course.ID, session.ID)
				if err != nil {
					reason = err.Error()
					continue
				}
				var id int64
				if err := tx.QueryRow(ctx, `INSERT INTO exams (course_id, session_id) VALUES ($1, $2) RETURNING id`, course.ID, session.ID).Scan(&id); err != nil {
					return value, err
				}
				if err := replaceRooms(ctx, tx, id, rooms); err != nil {
					return value, err
				}
				result.IDs = append(result.IDs, id)
				state, err = loadSchedule(ctx, tx)
				if err != nil {
					return value, err
				}
				placed = true
				break
			}
			if !placed {
				return value, conflict("planning_unresolved", "No placement found for course %d: %s No changes were committed; another ordering may succeed.", course.ID, reason)
			}
		}
		value.Invalidate = len(result.IDs) > 0
		value.New, err = scheduleDocument(ctx, tx)
		return value, err
	})
	if err != nil {
		return BatchResult{IDs: []int64{}}, err
	}
	result.Count = len(result.IDs)
	return result, nil
}

func (s Scheduler) Staff(ctx context.Context, actorID int64, scope string, allocationID int64) (BatchResult, error) {
	result := BatchResult{IDs: []int64{}}
	if scope != "all" && scope != "one" || scope == "one" && allocationID < 1 || scope == "all" && allocationID != 0 {
		return result, fmt.Errorf("invalid staffing scope")
	}
	err := s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "automatic_staffing", Entity: "invigilator_assignments"}
		var err error
		value.Old, err = scheduleDocument(ctx, tx)
		if err != nil {
			return value, err
		}
		state, err := loadSchedule(ctx, tx)
		if err != nil {
			return value, err
		}
		if err := state.validate(false); err != nil {
			return value, err
		}
		assigned := map[int64]bool{}
		for _, assignment := range state.Assignments {
			assigned[assignment.ID] = true
		}
		found := scope == "all"
		for _, allocation := range state.Allocations {
			if scope == "one" && allocation.ID != allocationID {
				continue
			}
			found = true
			if assigned[allocation.ID] {
				if scope == "one" {
					return value, conflict("allocation_staffed", "Allocation %d already has an invigilator.", allocationID)
				}
				continue
			}
			if err := ctx.Err(); err != nil {
				return value, err
			}
			personnelID, err := state.selectPersonnel(allocation.ID)
			if err != nil {
				return value, err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO invigilator_assignments (id, personnel_id) VALUES ($1, $2)`, allocation.ID, personnelID); err != nil {
				return value, err
			}
			state.Assignments = append(state.Assignments, assignmentState{ID: allocation.ID, PersonnelID: personnelID})
			result.IDs = append(result.IDs, allocation.ID)
		}
		if !found {
			return value, ErrNotFound
		}
		value.Invalidate = len(result.IDs) > 0
		value.New, err = scheduleDocument(ctx, tx)
		return value, err
	})
	if err != nil {
		return BatchResult{IDs: []int64{}}, err
	}
	result.Count = len(result.IDs)
	return result, nil
}

func (s Scheduler) Clear(ctx context.Context, actorID int64) error {
	return s.mutate(ctx, actorID, func(tx pgx.Tx) (change, error) {
		value := change{Action: "clear", Entity: "exams", Invalidate: true}
		var err error
		value.Old, err = scheduleDocument(ctx, tx)
		if err != nil {
			return value, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM exams`); err != nil {
			return value, err
		}
		value.New, err = scheduleDocument(ctx, tx)
		return value, err
	})
}
