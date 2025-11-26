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

func NewSingleValueIntRange(n int) IntRange {
	return IntRange{
		Min:          n,
		MinInclude:   true,
		Max:          n,
		MaxInclude:   true,
		MinUnbounded: false,
		MaxUnbounded: false,
	}
}

func NewGreaterThanIntRange(n int) IntRange {
	return IntRange{
		Min:          n,
		MinInclude:   false,
		Max:          0,
		MaxInclude:   false,
		MinUnbounded: false,
		MaxUnbounded: true,
	}
}

func NewGreaterOrEqualThanIntRange(n int) IntRange {
	return IntRange{
		Min:          n,
		MinInclude:   true,
		Max:          0,
		MaxInclude:   false,
		MinUnbounded: false,
		MaxUnbounded: true,
	}
}

func NewLessThanIntRange(n int) IntRange {
	return IntRange{
		Min:          0,
		MinInclude:   false,
		Max:          n,
		MaxInclude:   false,
		MinUnbounded: true,
		MaxUnbounded: false,
	}
}

func NewLessOrEqualThanIntRange(n int) IntRange {
	return IntRange{
		Min:          0,
		MinInclude:   false,
		Max:          n,
		MaxInclude:   true,
		MinUnbounded: true,
		MaxUnbounded: false,
	}
}

func NewInclusiveIntRange(min int, max int) IntRange {
	return IntRange{
		Min:          min,
		MinInclude:   true,
		Max:          max,
		MaxInclude:   true,
		MinUnbounded: false,
		MaxUnbounded: false,
	}
}

func NewExclusiveIntRange(min int, max int) IntRange {
	return IntRange{
		Min:          min,
		MinInclude:   false,
		Max:          max,
		MaxInclude:   false,
		MinUnbounded: false,
		MaxUnbounded: false,
	}
}

func NewBoundedIntRange(min int, minInclude bool, max int, maxInclude bool) IntRange {
	return IntRange{
		Min:          min,
		MinInclude:   minInclude,
		Max:          max,
		MaxInclude:   maxInclude,
		MinUnbounded: false,
		MaxUnbounded: false,
	}
}

// IsValid returns whether the IntRange represents a non-empty, semantically valid interval.
//
// Rules:
//   - If a side is unbounded it must be open (cannot include infinity).
//   - If both sides are bounded and Min > Max then it's invalid.
//   - If Min == Max then it's valid only when both ends are inclusive (represents a single point).
//   - If Min and Max differ by 1 (e.g. (N,N+1)) then it's invalid if both ends are exclusive
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
		} else if r.Min == r.Max {
			return r.MinInclude && r.MaxInclude
		} else if r.Max - r.Min == 1 {
			// (N,N+1) 是空集
			if (!r.MinInclude && !r.MaxInclude) {
				return false
			}
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

// Lowest returns the lowest integer included in the IntRange and a boolean indicating whether such a value exists.
func (r IntRange) Lowest() (int, bool) {
	if r.MinUnbounded {
		return 0, false
	}
	if r.MinInclude {
		return r.Min, true
	}
	return r.Min + 1, true
}

// Highest returns the highest integer included in the IntRange and a boolean indicating whether such a value exists.
func (r IntRange) Highest() (int, bool) {
	if r.MaxUnbounded {
		return 0, false
	}
	if r.MaxInclude {
		return r.Max, true
	}
	return r.Max - 1, true
}

// HasIntesect reports whether the IntRange r intersects with another IntRange.
//
// It returns true if there exists at least one integer that is included in both ranges.
// If either range is invalid, it returns false.
func (r IntRange) HasIntesect(other IntRange) bool {
	if !r.IsValid() || !other.IsValid() {
		return false
	}

	// 检查是否没有交集的情况
	// r 在 other 左侧
	if !r.MaxUnbounded && !other.MinUnbounded {
		rMax, _ := r.Highest()
		otherMin, _ := other.Lowest()
		if rMax < otherMin {
			return false
		}
	}
	// r 在 other 右侧
	if !r.MinUnbounded && !other.MaxUnbounded {
		rMin, _ := r.Lowest()
		otherMax, _ := other.Highest()
		if rMin > otherMax {
			return false
		}
	}

	return true
}

func (r IntRange) Intersect(other IntRange) (IntRange) {
	// 计算交集的下界
	var newMin int
	var newMinInclude bool
	var newMinUnbounded bool
	if r.MinUnbounded && other.MinUnbounded {
		newMinUnbounded = true
	} else if r.MinUnbounded {
		newMin = other.Min
		newMinInclude = other.MinInclude
		newMinUnbounded = false
	} else if other.MinUnbounded {
		newMin = r.Min
		newMinInclude = r.MinInclude
		newMinUnbounded = false
	} else {
		if r.Min > other.Min {
			newMin = r.Min
			newMinInclude = r.MinInclude
		} else if r.Min < other.Min {
			newMin = other.Min
			newMinInclude = other.MinInclude
		} else {
			newMin = r.Min
			newMinInclude = r.MinInclude
		}
	}

	// 计算交集的上界
	var newMax int
	var newMaxInclude bool
	var newMaxUnbounded bool
	if r.MaxUnbounded && other.MaxUnbounded {
		newMaxUnbounded = true
	} else if r.MaxUnbounded {
		newMax = other.Max
		newMaxInclude = other.MaxInclude
		newMaxUnbounded = false
	} else if other.MaxUnbounded {
		newMax = r.Max
		newMaxInclude = r.MaxInclude
		newMaxUnbounded = false
	} else {
		if r.Max < other.Max {
			newMax = r.Max
			newMaxInclude = r.MaxInclude
		} else if r.Max > other.Max {
			newMax = other.Max
			newMaxInclude = other.MaxInclude
		} else {
			newMax = r.Max
			newMaxInclude = r.MaxInclude
		}
	}

	return IntRange{
		Min:          newMin,
		MinInclude:   newMinInclude,
		MinUnbounded: newMinUnbounded,
		Max:          newMax,
		MaxInclude:   newMaxInclude,
		MaxUnbounded: newMaxUnbounded,
	}
}


// Substract returns the parts of IntRange r that are not covered by other.
//
// It returns a slice of IntRange representing the portions of r that do not overlap with other.
// If there is no overlap, the result contains only r. If r is completely covered by other,
// the result is an empty slice.
func (r IntRange) Substract(other IntRange) []IntRange {
	var results []IntRange

	if !r.HasIntesect(other) {
		results = append(results, r)
		return results
	}

	// 计算左侧剩余部分
	if !other.MinUnbounded {
		rMin, bounded := r.Lowest()
		otherMin, _ := other.Lowest()
		if !bounded || rMin < otherMin {
			newRange := IntRange{
				Min:          r.Min,
				MinInclude:   r.MinInclude,
				MinUnbounded: r.MinUnbounded,
				Max:          other.Min,
				MaxInclude:   !other.MinInclude,
				MaxUnbounded: false,
			}
			if newRange.IsValid() {
				results = append(results, newRange)
			}
		}
	}

	// 计算右侧剩余部分
	if !other.MaxUnbounded {
		rMax, bounded := r.Highest()
		otherMax, _ := other.Highest()
		if !bounded || rMax > otherMax {
			newRange := IntRange{
				Min:          other.Max,
				MinInclude:   !other.MaxInclude,
				MinUnbounded: false,
				Max:          r.Max,
				MaxInclude:   r.MaxInclude,
				MaxUnbounded: r.MaxUnbounded,
			}
			if newRange.IsValid() {
				results = append(results, newRange)
			}
		}
	}

	return results
}

// TryMyBestToClosedInterval attempts to convert the IntRange into a closed interval representation.
//
// It returns a new IntRange that represents the same set of integers as the original range,
// but expressed in a closed interval form where possible. If both bounds are present, it returns
// an inclusive range [Min, Max]. If only one bound is present, it returns a half-infinite range
// (e.g., [Min, +∞) or (-∞, Max]). If neither bound is present, it returns a completely unbounded range.
func (r IntRange) TryMyBestToClosedInterval() IntRange {
	lowest, ok1 := r.Lowest()
	highest, ok2 := r.Highest()
	if ok1 && ok2 {
		return NewInclusiveIntRange(lowest, highest)
	} else if ok1 {
		return NewGreaterOrEqualThanIntRange(lowest)
	} else if ok2 {
		return NewLessOrEqualThanIntRange(highest)
	} else {
		return NewUnboundedIntRange()
	}
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
