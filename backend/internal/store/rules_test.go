package store

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func ruleCode(err error) string {
	var rule *RuleError
	if errors.As(err, &rule) {
		return rule.Code
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func fixture() scheduleState {
	s := scheduleState{
		Courses:     []courseState{{1, 1, 5, 100}, {2, 1, 5, 60}, {3, 2, 3, 80}},
		Rooms:       []roomState{{1, 150, 0, true}, {2, 100, 1, true}, {3, 80, 1, true}},
		Personnel:   []personnelState{{1, 1}, {2, 1}, {3, 2}},
		Exams:       []examState{{1, 1, 1}, {2, 2, 3}, {3, 3, 1}},
		Allocations: []allocationState{{1, 1, 1}, {2, 2, 1}, {3, 3, 2}},
		Assignments: []assignmentState{{1, 1}, {2, 2}, {3, 3}},
	}
	for i := 1; i <= 7; i++ {
		s.Sessions = append(s.Sessions, sessionState{int64(i), "2026-11-02", fmt.Sprintf("%02d:00:00", 7+i), fmt.Sprintf("%02d:45:00", 7+i)})
	}
	return s
}

func TestFinalStateRules(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		edit       func(*scheduleState)
	}{
		{"valid separated cohort and different department", "", func(s *scheduleState) {}},
		{"duplicate course", "course_once", func(s *scheduleState) { s.Exams[2].CourseID = 1 }},
		{"same cohort session", "cohort_gap", func(s *scheduleState) { s.Exams[1].SessionID = 1 }},
		{"adjacent cohort session", "cohort_gap", func(s *scheduleState) { s.Exams[1].SessionID = 2 }},
		{"third daily exam", "cohort_daily_limit", func(s *scheduleState) { s.Exams[2].SessionID = 5; s.Courses[2].DepartmentID = 1; s.Courses[2].Term = 5 }},
		{"cohort changed by master edit", "cohort_daily_limit", func(s *scheduleState) { s.Courses[2].DepartmentID = 1; s.Courses[2].Term = 5 }},
		{"overlapping sessions", "session_overlap", func(s *scheduleState) { s.Sessions[0].EndTime = "09:30:00" }},
		{"sessions may touch", "", func(s *scheduleState) { s.Sessions[0].EndTime = "09:00:00" }},
		{"same time on different dates", "", func(s *scheduleState) {
			s.Sessions = append(s.Sessions, sessionState{8, "2026-11-03", "08:00:00", "08:45:00"})
		}},
		{"room double booking", "room_conflict", func(s *scheduleState) { s.Allocations[2].ClassroomID = 1 }},
		{"inactive allocated room", "room_inactive", func(s *scheduleState) { s.Rooms[0].Active = false }},
		{"reduced room capacity", "room_capacity", func(s *scheduleState) { s.Rooms[0].Capacity = 50 }},
		{"increased enrollment", "room_capacity", func(s *scheduleState) { s.Courses[0].Enrollment = 151 }},
		{"missing room allocation", "room_capacity", func(s *scheduleState) { s.Allocations = s.Allocations[1:] }},
		{"staff double booking", "personnel_conflict", func(s *scheduleState) { s.Assignments[2].PersonnelID = 1 }},
		{"approved full day", "personnel_unavailable", func(s *scheduleState) {
			s.Requests = []requestState{{ID: 1, PersonnelID: 1, Date: "2026-11-02", Status: "approved"}}
		}},
		{"approved specific session", "personnel_unavailable", func(s *scheduleState) {
			id := int64(1)
			s.Requests = []requestState{{1, 1, "2026-11-02", &id, "approved"}}
		}},
		{"approved other session", "", func(s *scheduleState) {
			id := int64(2)
			s.Requests = []requestState{{1, 1, "2026-11-02", &id, "approved"}}
		}},
		{"pending does not block", "", func(s *scheduleState) {
			s.Requests = []requestState{{ID: 1, PersonnelID: 1, Date: "2026-11-02", Status: "pending"}}
		}},
		{"rejected does not block", "", func(s *scheduleState) {
			s.Requests = []requestState{{ID: 1, PersonnelID: 1, Date: "2026-11-02", Status: "rejected"}}
		}},
		{"request follows session date", "request_date", func(s *scheduleState) {
			id := int64(1)
			s.Requests = []requestState{{1, 1, "2026-11-03", &id, "pending"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture()
			tc.edit(&s)
			if code := ruleCode(s.validate(false)); code != tc.code {
				t.Fatalf("got %q, want %q", code, tc.code)
			}
		})
	}
}

func dutyFixture(positions ...int64) scheduleState {
	s := fixture()
	s.Courses, s.Exams, s.Allocations, s.Assignments = nil, nil, nil, nil
	for i, position := range positions {
		id := int64(i + 1)
		s.Courses = append(s.Courses, courseState{id, id, 1, 50})
		s.Exams = append(s.Exams, examState{id, id, position})
		s.Allocations = append(s.Allocations, allocationState{id, id, 1})
		s.Assignments = append(s.Assignments, assignmentState{id, 1})
	}
	return s
}

func TestStaffingBoundariesAndPublication(t *testing.T) {
	for _, tc := range []struct {
		positions []int64
		code      string
	}{
		{[]int64{1, 2, 3}, ""},
		{[]int64{1, 2, 4, 5}, ""},
		{[]int64{1, 2, 3, 4}, "personnel_consecutive_limit"},
		{[]int64{1, 3, 4, 2}, "personnel_consecutive_limit"},
		{[]int64{1, 2, 4, 5, 7}, "personnel_daily_limit"},
	} {
		s := dutyFixture(tc.positions...)
		if code := ruleCode(s.validate(false)); code != tc.code {
			t.Fatalf("positions %v: got %q, want %q", tc.positions, code, tc.code)
		}
	}
	s := fixture()
	s.Assignments = s.Assignments[1:]
	if err := s.validate(false); err != nil {
		t.Fatal(err)
	}
	if code := ruleCode(s.validate(true)); code != "unstaffed_room" {
		t.Fatalf("unstaffed schedule published: %s", code)
	}
}

func TestRoomSelectionMinimizesCountBeforeFloor(t *testing.T) {
	for _, tc := range []struct {
		name       string
		enrollment int
		rooms      []roomState
		want       []int64
		code       string
	}{
		{"one large room beats two same-floor rooms", 140, []roomState{{1, 150, 0, true}, {2, 80, 1, true}, {3, 80, 1, true}}, []int64{1}, ""},
		{"prefer one floor at equal count", 140, []roomState{{1, 100, 0, true}, {2, 90, 1, true}, {3, 60, 1, true}}, []int64{2, 3}, ""},
		{"several floors required", 180, []roomState{{1, 100, 0, true}, {2, 90, 1, true}, {3, 60, 1, true}}, []int64{1, 2}, ""},
		{"stable floor and ID ties", 100, []roomState{{9, 60, 2, true}, {4, 60, 1, true}, {3, 60, 1, true}, {1, 60, 2, true}}, []int64{3, 4}, ""},
		{"inactive excluded", 100, []roomState{{1, 150, 0, false}, {2, 60, 1, true}, {3, 60, 1, true}}, []int64{2, 3}, ""},
		{"shortage", 200, []roomState{{1, 150, 0, true}}, nil, "room_capacity"},
		{"no rooms", 100, nil, nil, "room_capacity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ids, err := chooseRooms(tc.enrollment, tc.rooms)
			if ruleCode(err) != tc.code || !reflect.DeepEqual(ids, tc.want) {
				t.Fatalf("rooms %v, error %v", ids, err)
			}
		})
	}
}

func TestDeterministicDepartmentFirstStaffing(t *testing.T) {
	s := fixture()
	s.Assignments = []assignmentState{{2, 1}}
	// Local person 2 is idle; person 1 has one duty. Shared person 3 is also idle.
	id, err := s.selectPersonnel(1)
	if err != nil || id != 2 {
		t.Fatalf("wanted local least-loaded person 2, got %d: %v", id, err)
	}
	s.Assignments = nil
	id, err = s.selectPersonnel(1)
	if err != nil || id != 1 {
		t.Fatalf("wanted stable ID tie-break 1, got %d: %v", id, err)
	}
	s.Requests = []requestState{{ID: 1, PersonnelID: 1, Date: "2026-11-02", Status: "approved"}, {ID: 2, PersonnelID: 2, Date: "2026-11-02", Status: "approved"}}
	id, err = s.selectPersonnel(1)
	if err != nil || id != 3 {
		t.Fatalf("wanted eligible shared person 3, got %d: %v", id, err)
	}
	s.Requests = append(s.Requests, requestState{ID: 3, PersonnelID: 3, Date: "2026-11-02", Status: "approved"})
	if _, err := s.selectPersonnel(1); ruleCode(err) != "personnel_unavailable" {
		t.Fatalf("unavailable staff selected: %v", err)
	}
}
