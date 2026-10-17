package store

import (
	"fmt"
	"sort"
)

// RuleError describes a conflict with the proposed final schedule.
type RuleError struct {
	Code, Message string
}

func (e *RuleError) Error() string { return e.Message }

func conflict(code, format string, values ...any) error {
	return &RuleError{Code: code, Message: fmt.Sprintf(format, values...)}
}

type courseState struct {
	ID           int64 `json:"id"`
	DepartmentID int64 `json:"department_id"`
	Term         int   `json:"term"`
	Enrollment   int   `json:"enrollment"`
}

type sessionState struct {
	ID        int64  `json:"id"`
	Date      string `json:"date"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type roomState struct {
	ID       int64 `json:"id"`
	Capacity int   `json:"capacity"`
	Floor    int   `json:"floor"`
	Active   bool  `json:"active"`
}

type personnelState struct {
	ID           int64 `json:"id"`
	DepartmentID int64 `json:"department_id"`
}

type examState struct {
	ID        int64 `json:"id"`
	CourseID  int64 `json:"course_id"`
	SessionID int64 `json:"session_id"`
}

type allocationState struct {
	ID          int64 `json:"id"`
	ExamID      int64 `json:"exam_id"`
	ClassroomID int64 `json:"classroom_id"`
}

type assignmentState struct {
	ID          int64 `json:"id"`
	PersonnelID int64 `json:"personnel_id"`
}

type requestState struct {
	ID          int64  `json:"id"`
	PersonnelID int64  `json:"personnel_id"`
	Date        string `json:"date"`
	SessionID   *int64 `json:"session_id"`
	Status      string `json:"status"`
}

type scheduleState struct {
	Courses     []courseState     `json:"courses"`
	Sessions    []sessionState    `json:"sessions"`
	Rooms       []roomState       `json:"rooms"`
	Personnel   []personnelState  `json:"personnel"`
	Exams       []examState       `json:"exams"`
	Allocations []allocationState `json:"allocations"`
	Assignments []assignmentState `json:"assignments"`
	Requests    []requestState    `json:"requests"`
}

func (s scheduleState) sessionOrder() (map[int64]sessionState, map[int64]int, error) {
	ordered := append([]sessionState(nil), s.Sessions...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Date != ordered[j].Date {
			return ordered[i].Date < ordered[j].Date
		}
		if ordered[i].StartTime != ordered[j].StartTime {
			return ordered[i].StartTime < ordered[j].StartTime
		}
		return ordered[i].ID < ordered[j].ID
	})
	sessions, positions := map[int64]sessionState{}, map[int64]int{}
	position := 0
	for i, current := range ordered {
		if current.StartTime >= current.EndTime {
			return nil, nil, conflict("session_time", "Session %d must end after it starts.", current.ID)
		}
		if i == 0 || ordered[i-1].Date != current.Date {
			position = 0
		} else {
			if ordered[i-1].EndTime > current.StartTime {
				return nil, nil, conflict("session_overlap", "Sessions %d and %d overlap.", ordered[i-1].ID, current.ID)
			}
			position++
		}
		sessions[current.ID], positions[current.ID] = current, position
	}
	return sessions, positions, nil
}

func (s scheduleState) validate(requireStaff bool) error {
	sessions, positions, err := s.sessionOrder()
	if err != nil {
		return err
	}
	courses := map[int64]courseState{}
	for _, course := range s.Courses {
		courses[course.ID] = course
	}
	rooms := map[int64]roomState{}
	for _, room := range s.Rooms {
		rooms[room.ID] = room
	}
	personnel := map[int64]bool{}
	for _, person := range s.Personnel {
		personnel[person.ID] = true
	}
	type cohortDay struct {
		department int64
		term       int
		date       string
	}
	cohorts := map[cohortDay][]int{}
	exams, usedCourses := map[int64]examState{}, map[int64]bool{}
	for _, exam := range s.Exams {
		course, courseExists := courses[exam.CourseID]
		session, sessionExists := sessions[exam.SessionID]
		if !courseExists || !sessionExists {
			return conflict("exam_reference", "Exam %d refers to a missing course or session.", exam.ID)
		}
		if usedCourses[exam.CourseID] {
			return conflict("course_once", "Course %d already has an exam.", exam.CourseID)
		}
		usedCourses[exam.CourseID], exams[exam.ID] = true, exam
		key := cohortDay{course.DepartmentID, course.Term, session.Date}
		prior := cohorts[key]
		if len(prior) >= 2 {
			return conflict("cohort_daily_limit", "Department %d term %d exceeds two exams on %s.", course.DepartmentID, course.Term, session.Date)
		}
		for _, position := range prior {
			distance := positions[session.ID] - position
			if distance >= -1 && distance <= 1 {
				return conflict("cohort_gap", "Department %d term %d needs a free session between exams on %s.", course.DepartmentID, course.Term, session.Date)
			}
		}
		cohorts[key] = append(prior, positions[session.ID])
	}
	type reservation struct{ session, room int64 }
	reserved, capacity := map[reservation]bool{}, map[int64]int64{}
	allocations := map[int64]allocationState{}
	for _, allocation := range s.Allocations {
		exam, examExists := exams[allocation.ExamID]
		room, roomExists := rooms[allocation.ClassroomID]
		if !examExists || !roomExists {
			return conflict("room_reference", "Allocation %d refers to a missing exam or room.", allocation.ID)
		}
		if !room.Active {
			return conflict("room_inactive", "Room %d is allocated but inactive.", room.ID)
		}
		key := reservation{exam.SessionID, room.ID}
		if reserved[key] {
			return conflict("room_conflict", "Room %d is already allocated in session %d.", room.ID, exam.SessionID)
		}
		reserved[key], allocations[allocation.ID] = true, allocation
		capacity[exam.ID] += int64(room.Capacity)
	}
	for _, exam := range s.Exams {
		if capacity[exam.ID] < int64(courses[exam.CourseID].Enrollment) {
			return conflict("room_capacity", "Exam %d does not have enough active room capacity.", exam.ID)
		}
	}
	for _, request := range s.Requests {
		if !personnel[request.PersonnelID] {
			return conflict("request_personnel", "Request %d refers to missing personnel.", request.ID)
		}
		if request.SessionID != nil {
			session, exists := sessions[*request.SessionID]
			if !exists || session.Date != request.Date {
				return conflict("request_date", "Request %d must use its session date.", request.ID)
			}
		}
	}
	type personDay struct {
		person int64
		date   string
	}
	duties, assigned := map[personDay][]int{}, map[int64]bool{}
	for _, assignment := range s.Assignments {
		allocation, exists := allocations[assignment.ID]
		if !exists || !personnel[assignment.PersonnelID] {
			return conflict("assignment_reference", "Assignment %d refers to missing personnel or allocation.", assignment.ID)
		}
		if assigned[assignment.ID] {
			return conflict("allocation_staffed", "Allocation %d already has an invigilator.", assignment.ID)
		}
		assigned[assignment.ID] = true
		session := sessions[exams[allocation.ExamID].SessionID]
		for _, request := range s.Requests {
			if request.Status == "approved" && request.PersonnelID == assignment.PersonnelID && request.Date == session.Date && (request.SessionID == nil || *request.SessionID == session.ID) {
				return conflict("personnel_unavailable", "Personnel %d has approved unavailability for session %d.", assignment.PersonnelID, session.ID)
			}
		}
		key := personDay{assignment.PersonnelID, session.Date}
		for _, position := range duties[key] {
			if position == positions[session.ID] {
				return conflict("personnel_conflict", "Personnel %d already has a duty in session %d.", assignment.PersonnelID, session.ID)
			}
		}
		duties[key] = append(duties[key], positions[session.ID])
		if len(duties[key]) > 4 {
			return conflict("personnel_daily_limit", "Personnel %d exceeds four duties on %s.", assignment.PersonnelID, session.Date)
		}
	}
	for _, assignment := range s.Assignments {
		allocation := allocations[assignment.ID]
		session := sessions[exams[allocation.ExamID].SessionID]
		day := duties[personDay{assignment.PersonnelID, session.Date}]
		sort.Ints(day)
		for i := 3; i < len(day); i++ {
			if day[i]-day[i-3] == 3 {
				return conflict("personnel_consecutive_limit", "Personnel %d cannot cover four consecutive sessions on %s.", assignment.PersonnelID, session.Date)
			}
		}
	}
	if requireStaff {
		for _, allocation := range s.Allocations {
			if !assigned[allocation.ID] {
				return conflict("unstaffed_room", "Allocation %d needs an invigilator before publication.", allocation.ID)
			}
		}
	}
	return nil
}

// chooseRooms minimizes count first, then chooses the lowest numbered feasible single floor.
func chooseRooms(enrollment int, available []roomState) ([]int64, error) {
	if enrollment < 1 {
		return nil, conflict("enrollment", "Enrollment must be positive.")
	}
	rooms := make([]roomState, 0, len(available))
	for _, room := range available {
		if room.Active {
			rooms = append(rooms, room)
		}
	}
	sort.Slice(rooms, func(i, j int) bool {
		if rooms[i].Capacity != rooms[j].Capacity {
			return rooms[i].Capacity > rooms[j].Capacity
		}
		return rooms[i].ID < rooms[j].ID
	})
	var capacity int64
	k := 0
	for k < len(rooms) && capacity < int64(enrollment) {
		capacity += int64(rooms[k].Capacity)
		k++
	}
	if capacity < int64(enrollment) {
		return nil, conflict("room_capacity", "Not enough free active room capacity for %d students.", enrollment)
	}
	selected := rooms[:k]
	byFloor := map[int][]roomState{}
	for _, room := range rooms {
		byFloor[room.Floor] = append(byFloor[room.Floor], room)
	}
	floors := make([]int, 0, len(byFloor))
	for floor := range byFloor {
		floors = append(floors, floor)
	}
	sort.Ints(floors)
	for _, floor := range floors {
		group := byFloor[floor]
		if len(group) < k {
			continue
		}
		var capacity int64
		for _, room := range group[:k] {
			capacity += int64(room.Capacity)
		}
		if capacity >= int64(enrollment) {
			selected = group[:k]
			break
		}
	}
	ids := make([]int64, len(selected))
	for i, room := range selected {
		ids[i] = room.ID
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (s scheduleState) selectPersonnel(allocationID int64) (int64, error) {
	var department int64
	found := false
	for _, allocation := range s.Allocations {
		if allocation.ID != allocationID {
			continue
		}
		found = true
		for _, exam := range s.Exams {
			if exam.ID != allocation.ExamID {
				continue
			}
			for _, course := range s.Courses {
				if course.ID == exam.CourseID {
					department = course.DepartmentID
				}
			}
		}
	}
	if !found {
		return 0, conflict("allocation_missing", "Allocation %d does not exist.", allocationID)
	}
	loads := map[int64]int{}
	for _, assignment := range s.Assignments {
		loads[assignment.PersonnelID]++
	}
	people := append([]personnelState(nil), s.Personnel...)
	sort.Slice(people, func(i, j int) bool {
		localI, localJ := people[i].DepartmentID == department, people[j].DepartmentID == department
		if localI != localJ {
			return localI
		}
		if loads[people[i].ID] != loads[people[j].ID] {
			return loads[people[i].ID] < loads[people[j].ID]
		}
		return people[i].ID < people[j].ID
	})
	for _, person := range people {
		candidate := s
		candidate.Assignments = append(append([]assignmentState(nil), s.Assignments...), assignmentState{ID: allocationID, PersonnelID: person.ID})
		if candidate.validate(false) == nil {
			return person.ID, nil
		}
	}
	return 0, conflict("personnel_unavailable", "No eligible invigilator is available for allocation %d.", allocationID)
}
