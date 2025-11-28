package bind

import "testing"

func TestRange_IsValid_DirectCases(t *testing.T) {
	cases := []struct {
		name string
		r    IntRange
		exp  bool
	}{
		{
			"valid_finite_inclusive",
			IntRange{Min: 1, MinInclude: true, Max: 3, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			true,
		},
		{
			"invalid_min_gt_max",
			IntRange{Min: 5, MinInclude: true, Max: 4, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			false,
		},
		{
			"single_point_both_inclusive",
			IntRange{Min: 7, MinInclude: true, Max: 7, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			true,
		},
		{
			"single_point_one_exclusive",
			IntRange{Min: 7, MinInclude: true, Max: 7, MaxInclude: false, MinUnbounded: false, MaxUnbounded: false},
			false,
		},
		{
			"min_infinite_but_inclusive_invalid",
			IntRange{Min: 0, MinInclude: true, Max: 10, MaxInclude: true, MinUnbounded: true, MaxUnbounded: false},
			false,
		},
		{
			"max_infinite_but_inclusive_invalid",
			IntRange{Min: -10, MinInclude: true, Max: 0, MaxInclude: true, MinUnbounded: false, MaxUnbounded: true},
			false,
		},
		{
			"infinite_open_valid",
			IntRange{Min: 0, MinInclude: false, Max: 0, MaxInclude: false, MinUnbounded: true, MaxUnbounded: true},
			true,
		},
	}

	for _, tc := range cases {
		if got := tc.r.IsNotEmpty(); got != tc.exp {
			t.Fatalf("%s: IsValid() = %v, want %v (range=%+v)", tc.name, got, tc.exp, tc.r)
		}
	}
}

func TestRange_Contains_DirectCases(t *testing.T) {
	cases := []struct {
		name string
		r    IntRange
		val  int
		exp  bool
	}{
		{
			"contains_within_inclusive",
			IntRange{Min: 1, MinInclude: true, Max: 3, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			1,
			true,
		},
		{
			"contains_exclusive_left",
			IntRange{Min: 1, MinInclude: false, Max: 3, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			1,
			false,
		},
		{
			"contains_exclusive_right",
			IntRange{Min: 1, MinInclude: true, Max: 3, MaxInclude: false, MinUnbounded: false, MaxUnbounded: false},
			3,
			false,
		},
		{
			"infinite_left_contains_negative",
			IntRange{Min: 0, MinInclude: false, Max: 0, MaxInclude: true, MinUnbounded: true, MaxUnbounded: false},
			-999999999,
			true,
		},
		{
			"infinite_right_contains_large",
			IntRange{Min: 100, MinInclude: false, Max: 0, MaxInclude: false, MinUnbounded: false, MaxUnbounded: true},
			1000000000,
			true,
		},
		{
			"invalid_range_contains_always_false",
			IntRange{Min: 5, MinInclude: true, Max: 4, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			5,
			false,
		},
		{
			"single_point_contains",
			IntRange{Min: 42, MinInclude: true, Max: 42, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			42,
			true,
		},
		{
			"single_point_other_false",
			IntRange{Min: 42, MinInclude: true, Max: 42, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			41,
			false,
		},
	}

	for _, tc := range cases {
		if got := tc.r.Contains(tc.val); got != tc.exp {
			t.Fatalf("%s: Contains(%d) = %v, want %v (range=%+v)", tc.name, tc.val, got, tc.exp, tc.r)
		}
	}
}

func TestNewRange_ErrorsForEmptyIntervalsAndInfiniteInclusivity(t *testing.T) {
	errCases := []string{
		"(1,1)", // empty because both exclusive
		"(1,1]", // empty because left exclusive when equal
		"[1,1)", // empty because right exclusive when equal
		"(,]",   // right inclusive but infinite -> invalid
		"[,)",   // left inclusive but infinite -> invalid
		"( , ]", // whitespace variants should also error
		"abc",   // nonsense
		"(",     // malformed
	}

	for _, s := range errCases {
		if _, err := NewIntRange(s, false); err == nil {
			t.Fatalf("expected NewRange(%q) to return error, got nil", s)
		}
	}
}

func TestNewRange_ParsesValidIntervalsAndOperators(t *testing.T) {
	cases := []struct {
		name string
		s    string
		exp  IntRange
	}{
		{
			"inclusive",
			"[1,3]",
			IntRange{Min: 1, MinInclude: true, Max: 3, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
		},
		{
			"exclusive",
			"(1,3)",
			IntRange{Min: 1, MinInclude: false, Max: 3, MaxInclude: false, MinUnbounded: false, MaxUnbounded: false},
		},
		{
			"left_infinite_right_inclusive",
			"(,5]",
			IntRange{Min: 0, MinInclude: false, Max: 5, MaxInclude: true, MinUnbounded: true, MaxUnbounded: false},
		},
		{
			"left_inclusive_right_infinite",
			"[3,)",
			IntRange{Min: 3, MinInclude: true, Max: 0, MaxInclude: false, MinUnbounded: false, MaxUnbounded: true},
		},
		{
			"whitespace",
			"[ -2 , 4 )",
			IntRange{Min: -2, MinInclude: true, Max: 4, MaxInclude: false, MinUnbounded: false, MaxUnbounded: false},
		},
		{
			"both_infinite_open",
			"(,)",
			IntRange{Min: 0, MinInclude: false, Max: 0, MaxInclude: false, MinUnbounded: true, MaxUnbounded: true},
		},
		{
			"plain_integer",
			"42",
			IntRange{Min: 42, MinInclude: true, Max: 42, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
		},
		{
			"equals_prefix",
			"=42",
			IntRange{Min: 42, MinInclude: true, Max: 42, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
		},
		{
			"greater_than",
			">5",
			IntRange{Min: 5, MinInclude: false, Max: 0, MaxInclude: false, MinUnbounded: false, MaxUnbounded: true},
		},
		{
			"greater_or_equal",
			">=5",
			IntRange{Min: 5, MinInclude: true, Max: 0, MaxInclude: false, MinUnbounded: false, MaxUnbounded: true},
		},
		{
			"less_than",
			"<10",
			IntRange{Min: 0, MinInclude: false, Max: 10, MaxInclude: false, MinUnbounded: true, MaxUnbounded: false},
		},
		{
			"less_or_equal",
			"<=10",
			IntRange{Min: 0, MinInclude: false, Max: 10, MaxInclude: true, MinUnbounded: true, MaxUnbounded: false},
		},
	}

	for _, tc := range cases {
		got, err := NewIntRange(tc.s, false)
		if err != nil {
			t.Fatalf("%s: NewRange(%q) returned error: %v", tc.name, tc.s, err)
		}
		if got.Min != tc.exp.Min ||
			got.Max != tc.exp.Max ||
			got.MinInclude != tc.exp.MinInclude ||
			got.MaxInclude != tc.exp.MaxInclude ||
			got.MinUnbounded != tc.exp.MinUnbounded ||
			got.MaxUnbounded != tc.exp.MaxUnbounded {
			t.Fatalf("%s: NewRange(%q) = %+v, want %+v", tc.name, tc.s, got, tc.exp)
		}
	}
}

// 新增：为 LessThan / LessOrEqualThan / GreaterThan / GreaterOrEqualThan 增加单元测试
func TestRange_ComparisonHelpers(t *testing.T) {
	cases := []struct {
		name string
		r    IntRange
		n    int
		less bool
		le   bool
		gt   bool
		ge   bool
	}{
		{
			"finite_inclusive_between",
			IntRange{Min: 1, MinInclude: true, Max: 3, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			5,
			true,  // maxAllowed 3 < 5
			true,  // 3 <= 5
			false, // minAllowed 1 > 5 ? no
			false, // 1 >= 5 ? no
		},
		{
			"max_equal_n",
			IntRange{Min: 1, MinInclude: true, Max: 4, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			4,
			false, // 4 < 4 false
			true,  // 4 <= 4 true
			false,
			false,
		},
		{
			"max_exclusive",
			IntRange{Min: 1, MinInclude: true, Max: 4, MaxInclude: false, MinUnbounded: false, MaxUnbounded: false},
			4,
			true, // maxAllowed = 3 < 4
			true, // 3 <= 4
			false,
			false,
		},
		{
			"min_exclusive",
			IntRange{Min: 5, MinInclude: false, Max: 10, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			4,
			false,
			false,
			true, // minAllowed = 6 > 4
			true, // 6 >= 4
		},
		{
			"min_equal_n",
			IntRange{Min: 5, MinInclude: true, Max: 10, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			5,
			false,
			false,
			false, // minAllowed = 5 > 5 false
			true,  // minAllowed >= 5 true
		},
		{
			"max_unbounded_should_false_for_less",
			IntRange{Min: 1, MinInclude: true, Max: 0, MaxInclude: false, MinUnbounded: false, MaxUnbounded: true},
			100,
			false, // cannot guarantee all elements < 100
			false,
			false,
			false,
		},
		{
			"min_unbounded_should_false_for_greater",
			IntRange{Min: 0, MinInclude: false, Max: 10, MaxInclude: true, MinUnbounded: true, MaxUnbounded: false},
			-1000,
			false,
			false,
			false, // cannot guarantee all elements > -1000
			false,
		},
		{
			"invalid_range_all_false",
			IntRange{Min: 5, MinInclude: true, Max: 4, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			10,
			false,
			false,
			false,
			false,
		},
	}

	for _, tc := range cases {
		if got := tc.r.LessThan(tc.n); got != tc.less {
			t.Fatalf("%s: LessThan(%d) = %v, want %v (range=%+v)", tc.name, tc.n, got, tc.less, tc.r)
		}
		if got := tc.r.LessOrEqualThan(tc.n); got != tc.le {
			t.Fatalf("%s: LessOrEqualThan(%d) = %v, want %v (range=%+v)", tc.name, tc.n, got, tc.le, tc.r)
		}
		if got := tc.r.GreaterThan(tc.n); got != tc.gt {
			t.Fatalf("%s: GreaterThan(%d) = %v, want %v (range=%+v)", tc.name, tc.n, got, tc.gt, tc.r)
		}
		if got := tc.r.GreaterOrEqualThan(tc.n); got != tc.ge {
			t.Fatalf("%s: GreaterOrEqualThan(%d) = %v, want %v (range=%+v)", tc.name, tc.n, got, tc.ge, tc.r)
		}
	}
}

func TestIntRange_Properties(t *testing.T) {
	cases := []struct {
		name         string
		r            IntRange
		single       bool
		singleVal    int
		isLess       bool
		isLessEq     bool
		isGreater    bool
		isGreaterEq  bool
		lowerBounded bool
		upperBounded bool
		unbounded    bool
	}{
		{
			"single_value",
			IntRange{Min: 5, MinInclude: true, Max: 5, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			true, 5, false, false, false, false, true, true, false,
		},
		{
			"single_value_not_both_inclusive",
			IntRange{Min: 5, MinInclude: true, Max: 5, MaxInclude: false, MinUnbounded: false, MaxUnbounded: false},
			false, 0, false, false, false, false, true, true, false,
		},
		{
			"less_than_open_right",
			IntRange{MinUnbounded: true, MinInclude: false, Max: 10, MaxInclude: false, MaxUnbounded: false},
			false, 0, true, false, false, false, false, true, false,
		},
		{
			"less_or_equal",
			IntRange{MinUnbounded: true, MinInclude: false, Max: 10, MaxInclude: true, MaxUnbounded: false},
			false, 0, false, true, false, false, false, true, false,
		},
		{
			"greater_than_open_left",
			IntRange{Min: 3, MinInclude: false, MinUnbounded: false, MaxUnbounded: true, MaxInclude: false},
			false, 0, false, false, true, false, true, false, false,
		},
		{
			"greater_or_equal_left",
			IntRange{Min: 3, MinInclude: true, MinUnbounded: false, MaxUnbounded: true, MaxInclude: false},
			false, 0, false, false, false, true, true, false, false,
		},
		{
			"both_unbounded",
			IntRange{MinUnbounded: true, MinInclude: false, MaxUnbounded: true, MaxInclude: false},
			false, 0, false, false, false, false, false, false, true,
		},
		{
			"finite_bounded_interval",
			IntRange{Min: 1, MinInclude: true, Max: 4, MaxInclude: false, MinUnbounded: false, MaxUnbounded: false},
			false, 0, false, false, false, false, true, true, false,
		},
	}

	for _, tc := range cases {
		if got := tc.r.IsSingleValue(); got != tc.single {
			t.Fatalf("%s: IsSingleValue() = %v, want %v (range=%+v)", tc.name, got, tc.single, tc.r)
		}
		if v, ok := tc.r.SingleValue(); ok != tc.single || (ok && v != tc.singleVal) {
			t.Fatalf("%s: SingleValue() = (%d,%v), want (%d,%v) (range=%+v)", tc.name, v, ok, tc.singleVal, tc.single, tc.r)
		}
		if got := tc.r.IsLessThan(); got != tc.isLess {
			t.Fatalf("%s: IsLessThan() = %v, want %v (range=%+v)", tc.name, got, tc.isLess, tc.r)
		}
		if got := tc.r.IsLessOrEqualThan(); got != tc.isLessEq {
			t.Fatalf("%s: IsLessOrEqualThan() = %v, want %v (range=%+v)", tc.name, got, tc.isLessEq, tc.r)
		}
		if got := tc.r.IsGreaterThan(); got != tc.isGreater {
			t.Fatalf("%s: IsGreaterThan() = %v, want %v (range=%+v)", tc.name, got, tc.isGreater, tc.r)
		}
		if got := tc.r.IsGreaterOrEqualThan(); got != tc.isGreaterEq {
			t.Fatalf("%s: IsGreaterOrEqualThan() = %v, want %v (range=%+v)", tc.name, got, tc.isGreaterEq, tc.r)
		}
		if got := tc.r.IsLowerBounded(); got != tc.lowerBounded {
			t.Fatalf("%s: IsLowerBounded() = %v, want %v (range=%+v)", tc.name, got, tc.lowerBounded, tc.r)
		}
		if got := tc.r.IsUpperBounded(); got != tc.upperBounded {
			t.Fatalf("%s: IsUpperBounded() = %v, want %v (range=%+v)", tc.name, got, tc.upperBounded, tc.r)
		}
		if got := tc.r.IsUnbounded(); got != tc.unbounded {
			t.Fatalf("%s: IsUnbounded() = %v, want %v (range=%+v)", tc.name, got, tc.unbounded, tc.r)
		}
	}
}

func TestIntRange_String(t *testing.T) {
	cases := []struct {
		name string
		r    IntRange
		exp  string
	}{
		{
			"single integer",
			IntRange{Min: 5, MinInclude: true, Max: 5, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			"5",
		},
		{
			"left infinite, right inclusive",
			IntRange{MinUnbounded: true, MinInclude: false, Max: 10, MaxInclude: true, MaxUnbounded: false},
			"<=10",
		},
		{
			"right infinite, left exclusive",
			IntRange{Min: 3, MinInclude: false, MinUnbounded: false, MaxUnbounded: true, MaxInclude: false},
			">3",
		},
		{
			"finite interval",
			IntRange{Min: 1, MinInclude: true, Max: 4, MaxInclude: false, MinUnbounded: false, MaxUnbounded: false},
			"[1,4)",
		},
		{
			"both infinite open",
			IntRange{MinUnbounded: true, MinInclude: false, MaxUnbounded: true, MaxInclude: false},
			"(-∞,∞)",
		},
	}

	for _, tc := range cases {
		if got := tc.r.String(); got != tc.exp {
			t.Fatalf("%s: String() = %q, want %q (range=%+v)", tc.name, got, tc.exp, tc.r)
		}
	}
}

func TestIntRange_ToParseableString(t *testing.T) {
	cases := []struct {
		name string
		r    IntRange
		exp  string
	}{
		{
			"single integer parseable",
			IntRange{Min: 5, MinInclude: true, Max: 5, MaxInclude: true, MinUnbounded: false, MaxUnbounded: false},
			"5",
		},
		{
			"left infinite parseable (operator form)",
			IntRange{MinUnbounded: true, MinInclude: false, Max: 10, MaxInclude: true, MaxUnbounded: false},
			"<=10",
		},
		{
			"right infinite parseable (operator form)",
			IntRange{Min: 3, MinInclude: false, MinUnbounded: false, MaxUnbounded: true, MaxInclude: false},
			">3",
		},
		{
			"finite interval parseable",
			IntRange{Min: 1, MinInclude: true, Max: 4, MaxInclude: false, MinUnbounded: false, MaxUnbounded: false},
			"[1,4)",
		},
		{
			"both infinite parseable (empty sides)",
			IntRange{MinUnbounded: true, MinInclude: false, MaxUnbounded: true, MaxInclude: false},
			"(,)",
		},
	}

	for _, tc := range cases {
		if got := tc.r.ToParseableString(); got != tc.exp {
			t.Fatalf("%s: ToParseableString() = %q, want %q (range=%+v)", tc.name, got, tc.exp, tc.r)
		}
	}
}
