package bind

import "testing"

func TestNewIntRangeFilter_ValidAndString(t *testing.T) {
	tests := []struct {
		name          string
		in            string
		wantStr       string
		wantNotEmpty  bool
		wantAll       bool
		testTrue      []int
		testFalse     []int
	}{
		{"empty", "", "", false, false, nil, nil},
		{"all", "all", "all", true, true, []int{0, 50, 100}, nil},
		{"single", "3", "3", true, false, []int{3}, []int{-1, 2, 4}},
		{"inclusive", "1-3", "1-3", true, false, []int{1, 2, 3}, []int{-1, 0, 4}},
		{"unbounded", "-5-", "all", true, true, []int{0, 5, 1000}, []int{-1}},
		{"rightOpen", "5-", "5-", true, false, []int{5, 1000}, []int{-1, 4}},
		{"leftOpen", "-4", "0-4", true, false, []int{0, 1, 2, 3, 4}, []int{-1, 5, 6}},
		{"multiple_singles", "1_3_5", "1_3_5", true, false, []int{1, 3, 5}, []int{-1, 0, 2, 4, 6}},
		{"multiple_ranges", "1-2_4-5", "1-2_4-5", true, false, []int{1, 2, 4, 5}, []int{-1, 0, 3, 6}},
		{"adjacent_singles_merge", "1_2", "1-2", true, false, []int{1, 2}, []int{-1, 0, 3}},
		{"adjacent_ranges_no_merge", "1-3_5-7", "1-3_5-7", true, false, []int{1, 2, 3, 5, 6, 7}, []int{-1, 0, 4, 8}},
		{"adjacent_ranges_merge", "1-3_4-6", "1-6", true, false, []int{1, 2, 3, 4, 5, 6}, []int{-1, 0, 7}},
		{"adjacent_range_and_single_merge", "1-3_4", "1-4", true, false, []int{1, 2, 3, 4}, []int{-1, 0, 5}},
		{"with_unbounded", " -3_5- ", "0-3_5-", true, false, []int{0, 1, 2, 3, 5, 10, 100}, []int{-1, 4}},
		{"compound_valid", "1_3-5_7-7", "1_3-5_7", true, false, []int{1, 3, 4, 5, 7}, []int{-1, 0, 2, 6, 8}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := NewNaturalRangeFilter(tc.in)
			if err != nil {
				t.Fatalf("unexpected error parsing %q: %v", tc.in, err)
			}
			if got := f.String(); got != tc.wantStr {
				t.Fatalf("String(): got %q want %q (input %q)", got, tc.wantStr, tc.in)
			}
			if ne := f.IsNotEmpty(); ne != tc.wantNotEmpty {
				t.Fatalf("IsNotEmpty(): got %v want %v (input %q)", ne, tc.wantNotEmpty, tc.in)
			}
			if ub := f.IsAllNatural(); ub != tc.wantAll {
				t.Fatalf("IsAllNatural(): got %v want %v (input %q)", ub, tc.wantAll, tc.in)
			}
			for _, v := range tc.testTrue {
				if !f.Test(v) {
					t.Fatalf("Test(%d) = false, want true (input %q)", v, tc.in)
				}
			}
			for _, v := range tc.testFalse {
				if f.Test(v) {
					t.Fatalf("Test(%d) = true, want false (input %q)", v, tc.in)
				}
			}
		})
	}
}

func TestNewIntRangeFilter_Errors(t *testing.T) {
	errCases := []struct {
		in  string
		sub string // substring expected in error (for a lightweight check)
	}{
		{"_", "empty token"},
		{"1__2", "empty token"},
		{"1--2", "invalid token"},
		{"3_1-4", "non-decreasing"},
		{"1_2_1", "non-decreasing"},
		{"-5_3", "non-decreasing"},
		{"a-b", "not natural number"},
		{"2-b", "not natural number"},
		{"a-3", "not natural number"},
		{"--", ""},
		{"-x-", "not natural number"},
		{"x", "not natural number"},
		{"1-2-3", "invalid token"},
		{"-1-3", "invalid token"},
		{"2--", "invalid token"},
	}

	for _, tc := range errCases {
		t.Run(tc.in, func(t *testing.T) {
			_, err := NewNaturalRangeFilter(tc.in)
			if err == nil {
				t.Fatalf("expected error parsing %q, got nil", tc.in)
			}
			if tc.sub != "" && !contains(err.Error(), tc.sub) {
				t.Fatalf("error for %q does not contain %q: %v", tc.in, tc.sub, err)
			}
		})
	}
}

// small helper to avoid importing strings just for one contains call
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(sub) > len(s) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
