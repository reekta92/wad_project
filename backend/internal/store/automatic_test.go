package store

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPlacementEvaluatesConflictsWithoutChangingState(t *testing.T) {
	s := fixture()
	s.Exams = append(s.Exams[:1:1], s.Exams[2:]...)
	s.Allocations = append(s.Allocations[:1:1], s.Allocations[2:]...)
	s.Assignments = append(s.Assignments[:1:1], s.Assignments[2:]...)
	before, _ := json.Marshal(s)
	if _, err := s.placement(2, 2); ruleCode(err) != "cohort_gap" {
		t.Fatalf("adjacent placement accepted: %v", err)
	}
	rooms, err := s.placement(2, 3)
	if err != nil || !reflect.DeepEqual(rooms, []int64{1}) {
		t.Fatalf("separated placement: %v %v", rooms, err)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("candidate evaluation mutated the existing schedule")
	}
}

func TestAvailabilityUsesTheSameDutyLimitsAsAssignments(t *testing.T) {
	for _, tc := range []struct {
		positions []int64
		session   int64
		code      string
	}{
		{[]int64{1, 2, 3}, 4, "personnel_consecutive_limit"},
		{[]int64{1, 2, 4, 5}, 7, "personnel_daily_limit"},
		{[]int64{1, 2, 4}, 5, ""},
		{[]int64{1}, 1, "personnel_conflict"},
	} {
		if err := dutyFixture(tc.positions...).personAvailable(1, tc.session, 0); ruleCode(err) != tc.code {
			t.Fatalf("%v plus %d: %v", tc.positions, tc.session, err)
		}
	}
	if err := fixture().personAvailable(1, 1, 1); err != nil {
		t.Fatalf("existing duty rejected during validation: %v", err)
	}
}
