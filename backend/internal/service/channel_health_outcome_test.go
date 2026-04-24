package service

import "testing"

func TestMapStatusToOutcome(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		want       HealthOutcome
	}{
		{"200 success", 200, OutcomeSuccess},
		{"204 success", 204, OutcomeSuccess},
		{"299 edge success", 299, OutcomeSuccess},
		{"300 not success", 300, OutcomeError},
		{"400 error", 400, OutcomeError},
		{"401 error (not special)", 401, OutcomeError},
		{"403 error (not special)", 403, OutcomeError},
		{"429 rate-limited", 429, OutcomeRateLimited},
		{"500 error", 500, OutcomeError},
		{"502 error", 502, OutcomeError},
		{"529 overloaded", 529, OutcomeOverloaded},
		{"zero treated as error", 0, OutcomeError},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := mapStatusToOutcome(tc.statusCode); got != tc.want {
				t.Fatalf("mapStatusToOutcome(%d) = %v, want %v", tc.statusCode, got, tc.want)
			}
		})
	}
}
