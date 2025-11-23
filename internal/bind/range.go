package bind

import (
	"fmt"
	"strconv"
	"strings"
)

type IntRange struct {
	Min          int
	MinInclude   bool
	Max          int
	MaxInclude   bool
	MinUnbounded bool // true 表示左端为 -inf
	MaxUnbounded bool // true 表示右端为 +inf
}

// NewIntRange parses value and returns an IntRange.
//
// Supported formats:
//   - N
//   - =N
//   - >N, >=N, <N, <=N
//   - (min,max), (min,max], [min,max), [min,max]
//   - ( ,max), (min, ), ( ,max] etc.
//
// Spaces are ignored. Unbounded sides are represented via MinUnbounded/MaxUnbounded.
//
// Rules:
//   - If a side is unbounded, that side must be an open interval (use '(' or ')' not '[' or ']').
//   - The parameter emptyAsUnbounded controls how an empty input string is handled:
//       * If emptyAsUnbounded == true and value == "", this function returns a completely unbounded range
//         (equivalent to "(,)" / NewUnboundedIntRange()).
//       * If emptyAsUnbounded == false and value == "", an error is returned.
//
// Examples:
//   NewIntRange("", true)  -> unbounded range
//   NewIntRange("", false) -> error
func NewIntRange(value string, emptyAsUnbounded bool) (IntRange, error) {

	if value == "" {
		if emptyAsUnbounded {
			return NewUnboundedIntRange(), nil
		} else {
			return IntRange{}, fmt.Errorf("empty range")
		}
	}
	s := strings.TrimSpace(value)

	// 初始化为无界开区间
	r := IntRange{
		Min:          0,
		MinInclude:   false,
		Max:          0,
		MaxInclude:   false,
		MinUnbounded: true,
		MaxUnbounded: true,
	}

	// helper to set exact value
	setExact := func(n int) {
		r.Min = n
		r.Max = n
		r.MinInclude = true
		r.MaxInclude = true
		r.MinUnbounded = false
		r.MaxUnbounded = false
	}

	// parse integer helper
	parseInt := func(tok string) (int, error) {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			return 0, fmt.Errorf("empty integer")
		}
		return strconv.Atoi(tok)
	}

	// prefix operators
	switch {
	case strings.HasPrefix(s, "="):
		n, err := parseInt(s[1:])
		if err != nil {
			return IntRange{}, fmt.Errorf("invalid =N: %w", err)
		}
		setExact(n)
		return r, nil
	case strings.HasPrefix(s, ">="):
		n, err := parseInt(s[2:])
		if err != nil {
			return IntRange{}, fmt.Errorf("invalid >=N: %w", err)
		}
		r.Min = n
		r.MinInclude = true
		r.MinUnbounded = false
		return r, nil
	case strings.HasPrefix(s, ">"):
		n, err := parseInt(s[1:])
		if err != nil {
			return IntRange{}, fmt.Errorf("invalid >N: %w", err)
		}
		r.Min = n
		r.MinInclude = false
		r.MinUnbounded = false
		return r, nil
	case strings.HasPrefix(s, "<="):
		n, err := parseInt(s[2:])
		if err != nil {
			return IntRange{}, fmt.Errorf("invalid <=N: %w", err)
		}
		r.Max = n
		r.MaxInclude = true
		r.MaxUnbounded = false
		return r, nil
	case strings.HasPrefix(s, "<"):
		n, err := parseInt(s[1:])
		if err != nil {
			return IntRange{}, fmt.Errorf("invalid <N: %w", err)
		}
		r.Max = n
		r.MaxInclude = false
		r.MaxUnbounded = false
		return r, nil
	}

	// interval notation
	if len(s) >= 2 && (s[0] == '(' || s[0] == '[') && (s[len(s)-1] == ')' || s[len(s)-1] == ']') {
		leftInclusive := s[0] == '['
		rightInclusive := s[len(s)-1] == ']'
		inner := strings.TrimSpace(s[1 : len(s)-1])
		parts := strings.SplitN(inner, ",", 2)
		if len(parts) != 2 {
			return IntRange{}, fmt.Errorf("invalid interval syntax: %s", value)
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])

		// left side
		if left == "" {
			// -inf; must be open
			if leftInclusive {
				return IntRange{}, fmt.Errorf("infinite side must be open on left: %s", value)
			}
			r.Min = 0
			r.MinInclude = false
			r.MinUnbounded = true
		} else {
			n, err := strconv.Atoi(left)
			if err != nil {
				return IntRange{}, fmt.Errorf("invalid left integer: %w", err)
			}
			r.Min = n
			r.MinInclude = leftInclusive
			r.MinUnbounded = false
		}

		// right side
		if right == "" {
			// +inf; must be open
			if rightInclusive {
				return IntRange{}, fmt.Errorf("infinite side must be open on right: %s", value)
			}
			r.Max = 0
			r.MaxInclude = false
			r.MaxUnbounded = true
		} else {
			n, err := strconv.Atoi(right)
			if err != nil {
				return IntRange{}, fmt.Errorf("invalid right integer: %w", err)
			}
			r.Max = n
			r.MaxInclude = rightInclusive
			r.MaxUnbounded = false
		}

		// validate consistency for bounded sides
		if !r.MinUnbounded && !r.MaxUnbounded {
			if r.Min > r.Max {
				return IntRange{}, fmt.Errorf("empty interval: min > max")
			}
			if r.Min == r.Max && (!r.MinInclude || !r.MaxInclude) {
				// e.g. (N,N), (N,N], [N,N) 等都 empty
				return IntRange{}, fmt.Errorf("empty interval for equal bounds but not both inclusive")
			}
		}
		return r, nil
	}

	// plain integer
	if n, err := strconv.Atoi(s); err == nil {
		setExact(n)
		return r, nil
	}

	return IntRange{}, fmt.Errorf("unrecognized range format: %s", value)
}

func NewUnboundedIntRange() IntRange {
	return IntRange{
		Min:          0,
		MinInclude:   false,
		Max:          0,
		MaxInclude:   false,
		MinUnbounded: true,
		MaxUnbounded: true,
	}
}

// IsValid returns whether the IntRange represents a non-empty, semantically valid interval.
//
// Rules:
//   - If a side is unbounded it must be open (cannot include infinity).
//   - If both sides are bounded and Min > Max then it's invalid.
//   - If Min == Max then it's valid only when both ends are inclusive (represents a single point).
func (r IntRange) IsValid() bool {
	// 无穷端必须为开区间
	if r.MinUnbounded && r.MinInclude {
		return false
	}
	if r.MaxUnbounded && r.MaxInclude {
		return false
	}

	// 如果两端都有界，检查大小关系
	if !r.MinUnbounded && !r.MaxUnbounded {
		if r.Min > r.Max {
			return false
		}
		if r.Min == r.Max {
			return r.MinInclude && r.MaxInclude
		}
	}

	return true
}

// Contains checks whether the given integer n is included in the IntRange.
// Returns false if the range itself is invalid or n is outside the interval.
func (r IntRange) Contains(n int) bool {
	if !r.IsValid() {
		return false
	}

	// 检查下界（若有界）
	if !r.MinUnbounded {
		if r.MinInclude {
			if n < r.Min {
				return false
			}
		} else {
			if n <= r.Min {
				return false
			}
		}
	}

	// 检查上界（若有界）
	if !r.MaxUnbounded {
		if r.MaxInclude {
			if n > r.Max {
				return false
			}
		} else {
			if n >= r.Max {
				return false
			}
		}
	}

	return true
}

// LessThan reports whether every integer in the IntRange r is strictly less than n.
//
// It returns true only when r is valid, has a bounded upper limit, and the effective
// maximum value of the range (respecting MaxInclude; i.e. Max if inclusive, Max-1 if exclusive)
// is strictly less than n. If the range is invalid or the upper bound is unbounded,
// LessThan returns false.
func (r IntRange) LessThan(n int) bool {
	if !r.IsValid() {
		return false
	}
	// 如果没有上界，则不可能全部小于 n
	if r.MaxUnbounded {
		return false
	}
	// 最大允许的整数
	maxAllowed := r.Max
	if !r.MaxInclude {
		maxAllowed = r.Max - 1
	}
	return maxAllowed < n
}

// LessOrEqualThan reports whether the range's effective upper bound is less than or equal to n.
//
//  1. It returns false if the range is not valid or if the upper bound is unbounded.
//  2. If MaxInclude is true the effective upper bound is r.Max; otherwise the effective upper bound is r.Max - 1.
//  3. The function compares that effective upper bound to n and returns true when the bound is <= n.
func (r IntRange) LessOrEqualThan(n int) bool {
	if !r.IsValid() {
		return false
	}
	if r.MaxUnbounded {
		return false
	}
	maxAllowed := r.Max
	if !r.MaxInclude {
		maxAllowed = r.Max - 1
	}
	return maxAllowed <= n
}

// GreaterThan reports whether every integer covered by the IntRange is strictly greater than n.
//
// It returns false when the range is invalid or when the lower bound is unbounded,
// since an unbounded minimum cannot guarantee that all values exceed n.
// When the lower bound is exclusive (MinInclude == false), the comparison uses Min+1
// as the smallest allowed integer; otherwise it uses Min. The function returns true
// only if that smallest allowed integer is greater than n.
func (r IntRange) GreaterThan(n int) bool {
	if !r.IsValid() {
		return false
	}
	// 如果没有下界，则不可能全部大于 n
	if r.MinUnbounded {
		return false
	}
	// 最小允许的整数
	minAllowed := r.Min
	if !r.MinInclude {
		minAllowed = r.Min + 1
	}
	return minAllowed > n
}

// GreaterOrEqualThan reports whether the IntRange's smallest possible value is greater than or equal to n.
//
// It returns false for invalid ranges or ranges with no lower bound. For an exclusive lower bound
// (MinInclude == false) the effective minimum is treated as Min+1. This check only considers the lower
// bound (i.e. whether the entire finite range is at or above n) and does not examine the upper bound.
func (r IntRange) GreaterOrEqualThan(n int) bool {
	if !r.IsValid() {
		return false
	}
	if r.MinUnbounded {
		return false
	}
	minAllowed := r.Min
	if !r.MinInclude {
		minAllowed = r.Min + 1
	}
	return minAllowed >= n
}

// IsSingleValue reports whether the range represents a single discrete integer value.
//
// It returns true when both lower and upper bounds are present (not unbounded),
// the minimum and maximum are equal, and both bounds are inclusive; otherwise it returns false.
func (r IntRange) IsSingleValue() bool {
	return !r.MinUnbounded && !r.MaxUnbounded && r.Min == r.Max && r.MinInclude && r.MaxInclude
}

// SingleValue returns the single integer represented by the IntRange and a boolean
//
// indicating whether the range contains exactly one value. If the range is a single
// value this method returns that value and true; otherwise it returns 0 and false.
func (r IntRange) SingleValue() (int, bool) {
	if r.IsSingleValue() {
		return r.Min, true
	}
	return 0, false
}

// IsLessThan reports whether the range represents an open-ended "less than"
//
// constraint (no lower bound and a strict upper bound). It returns true when
// the minimum is unbounded, the maximum is bounded, and the maximum is
// exclusive (MinUnbounded == true, MaxUnbounded == false, MaxInclude == false).
func (r IntRange) IsLessThan() bool {
	return r.MinUnbounded && !r.MaxUnbounded && !r.MaxInclude
}

// IsLessOrEqualThan reports whether the range represents a "less than or equal to" interval.
//
// That is, the range has no lower bound and has a finite, inclusive upper bound (e.g. (-∞, b]).
// It returns true when the minimum is unbounded, the maximum is bounded, and the maximum is inclusive.
func (r IntRange) IsLessOrEqualThan() bool {
	return r.MinUnbounded && !r.MaxUnbounded && r.MaxInclude
}

// IsGreaterThan reports whether the range represents all values strictly greater
// than a finite lower bound (i.e., an exclusive lower bound with no upper bound).
//
// It returns true when the range has a bounded, exclusive minimum and is unbounded
// above — for example, the interval (min, +Inf).
func (r IntRange) IsGreaterThan() bool {
	return !r.MinUnbounded && !r.MinInclude && r.MaxUnbounded
}

// IsGreaterOrEqualThan reports whether the range represents the half-infinite interval [min, +∞).
//
// It returns true when the minimum is bounded and inclusive (MinUnbounded == false && MinInclude == true)
// and the maximum is unbounded (MaxUnbounded == true). Otherwise it returns false.
func (r IntRange) IsGreaterOrEqualThan() bool {
	return !r.MinUnbounded && r.MinInclude && r.MaxUnbounded
}

// IsLowerBounded reports whether the range has a defined lower bound.
//
// It returns true when the range is bounded below (i.e. MinUnbounded is false),
// meaning the Min field represents a valid lower bound. It returns false when
// the range is unbounded below (extends to negative infinity).
func (r IntRange) IsLowerBounded() bool {
	return !r.MinUnbounded
}

// IsUpperBounded reports whether the IntRange has an explicit upper bound.
//
// It returns true when the range's MaxUnbounded flag is false (i.e., a maximum is defined).
// If MaxUnbounded is true, the range is unbounded above and this returns false.
func (r IntRange) IsUpperBounded() bool {
	return !r.MaxUnbounded
}

// IsUnbounded reports whether the range has neither a lower nor an upper bound.
//
// It returns true when both MinUnbounded and MaxUnbounded are true, indicating
// the range is completely unbounded.
func (r IntRange) IsUnbounded() bool {
	return r.MinUnbounded && r.MaxUnbounded
}

// format returns a string representation of the range.
// If showInfty=true it uses "∞"/"-∞" for unbounded sides; if false it leaves unbounded sides empty
// (useful for producing parseable strings consumed by NewRange).
//
// Rules:
//  1. If the range is a single integer (bounded and Min==Max and both ends inclusive), return "N".
//  2. If exactly one side is unbounded and the other side is bounded, return operator form: ">N" / ">=N" / "<N" / "<=N".
//  3. Otherwise use interval notation like "[min,max)". When showInfty=true unbounded sides display "-∞" or "∞",
//     otherwise they are left empty.
func (r IntRange) format(showInfty bool) string {
	// single integer
	if !r.MinUnbounded && !r.MaxUnbounded && r.Min == r.Max && r.MinInclude && r.MaxInclude {
		return strconv.Itoa(r.Min)
	}

	// one-sided infinite -> operator form
	if r.MinUnbounded && !r.MaxUnbounded {
		if r.MaxInclude {
			return fmt.Sprintf("<=%d", r.Max)
		}
		return fmt.Sprintf("<%d", r.Max)
	}
	if r.MaxUnbounded && !r.MinUnbounded {
		if r.MinInclude {
			return fmt.Sprintf(">=%d", r.Min)
		}
		return fmt.Sprintf(">%d", r.Min)
	}

	// interval form
	leftB := "("
	if r.MinInclude {
		leftB = "["
	}
	rightB := ")"
	if r.MaxInclude {
		rightB = "]"
	}

	var leftStr, rightStr string
	if r.MinUnbounded {
		if showInfty {
			leftStr = "-∞"
		} else {
			leftStr = ""
		}
	} else {
		leftStr = strconv.Itoa(r.Min)
	}
	if r.MaxUnbounded {
		if showInfty {
			rightStr = "∞"
		} else {
			rightStr = ""
		}
	} else {
		rightStr = strconv.Itoa(r.Max)
	}

	return fmt.Sprintf("%s%s,%s%s", leftB, leftStr, rightStr, rightB)
}

// String implements fmt.Stringer, returning a human-friendly representation (unbounded sides shown with ∞).
func (r IntRange) String() string {
	return r.format(true)
}

// ToParseableString returns a string that can be parsed back by NewRange (unbounded sides left empty).
func (r IntRange) ToParseableString() string {
	return r.format(false)
}
