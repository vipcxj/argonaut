package bind

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// NaturalRangeFilter represents a filter composed of one or more IntRange elements (combined with logical OR).
// - Ranges holds the individual IntRange values that the filter accepts.
// - A value n is accepted by the filter if it falls within any of the IntRange elements in Ranges.
// - If Ranges is empty, the filter accepts no values.
// - Only natural numbers (0, 1, 2, ...) are considered valid inputs for testing against the filter.
type NaturalRangeFilter struct {
	Ranges []IntRange
}

// NewNaturalRangeFilter parses the string v and returns an NaturalRangeFilter or an error.
//
// Syntax (tokens are separated by underscore '_' characters):
//
//	"all"    -> matches all natural numbers (unbounded)
//	"N"      -> a single natural number
//	"N-M"    -> closed interval [N, M]
//	"N-"     -> >= N
//	"-M"     -> <= M
//
// Constraint: all numeric values encountered during parsing must be non-decreasing
// (they may be equal) when read left to right. For example, "1_3-5_7-7" is valid
// (1 < 3 < 5 < 7 = 7), whereas "3_1-4" or "1_2_1" are invalid. This constraint
// simplifies parsing and avoids ambiguous semantics.
//
// NewNaturalRangeFilter returns a parsed NaturalRangeFilter or an error describing why parsing failed.
func NewNaturalRangeFilter(v string) (NaturalRangeFilter, error) {
	var af NaturalRangeFilter
	v = strings.TrimSpace(v)
	if v == "" {
		return af, nil
	}
	if v == "all" {
		af.Ranges = []IntRange{
			NewUnboundedIntRange(),
		}
		return af, nil
	}

	tokens := strings.Split(v, "_")
	prev := 0
	for i, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			return NaturalRangeFilter{}, fmt.Errorf("empty token at position %d", i)
		}

		if strings.Count(tok, "-") > 1 {
			if tok != "--" && strings.HasPrefix(tok, "-") && strings.HasSuffix(tok, "-") {
				if _, err := parseNaturalNumber(tok[1 : len(tok)-1]); err != nil {
					return NaturalRangeFilter{}, fmt.Errorf("invalid token %q: %w", tok, err)
				} else {
					return NewAllNaturalRangeFilter(), nil
				}
			} else {
				return NaturalRangeFilter{}, fmt.Errorf("invalid token %q", tok)
			}
		}

		if strings.Contains(tok, "-") {
			sep := strings.Index(tok, "-")
			left := tok[:sep]
			right := tok[sep+1:]

			if left == "" && right == "" {
				return NaturalRangeFilter{}, fmt.Errorf("invalid token %q", tok)
			}

			switch {
			case left != "" && right != "":
				n1, err := parseNaturalNumber(left)
				if err != nil {
					return NaturalRangeFilter{}, fmt.Errorf("invalid left bound in %q: %w", tok, err)
				}
				n2, err := parseNaturalNumber(right)
				if err != nil {
					return NaturalRangeFilter{}, fmt.Errorf("invalid right bound in %q: %w", tok, err)
				}
				if n1 > n2 {
					return NaturalRangeFilter{}, fmt.Errorf("invalid range %q: min > max", tok)
				}
				if n1 < prev {
					return NaturalRangeFilter{}, fmt.Errorf("numbers must be non-decreasing: %d < %d", n1, prev)
				}
				if n2 < prev {
					return NaturalRangeFilter{}, fmt.Errorf("numbers must be non-decreasing: %d < %d", n2, prev)
				}
				af.Ranges = append(af.Ranges, NewInclusiveIntRange(n1, n2))
				prev = n2
			case left != "": // "N-"
				n, err := parseNaturalNumber(left)
				if err != nil {
					return NaturalRangeFilter{}, fmt.Errorf("invalid bound in %q: %w", tok, err)
				}
				if n < prev {
					return NaturalRangeFilter{}, fmt.Errorf("numbers must be non-decreasing: %d < %d", n, prev)
				}
				af.Ranges = append(af.Ranges, NewGreaterOrEqualThanIntRange(n))
				prev = n
			default: // "-M"
				n, err := parseNaturalNumber(right)
				if err != nil {
					return NaturalRangeFilter{}, fmt.Errorf("invalid bound in %q: %w", tok, err)
				}
				if n < prev {
					return NaturalRangeFilter{}, fmt.Errorf("numbers must be non-decreasing: %d < %d", n, prev)
				}
				af.Ranges = append(af.Ranges, NewLessOrEqualThanIntRange(n))
				prev = n
			}
			continue
		}

		// 单个正整数
		n, err := parseNaturalNumber(tok)
		if err != nil {
			return NaturalRangeFilter{}, fmt.Errorf("invalid token %q: %w", tok, err)
		}
		if n < prev {
			return NaturalRangeFilter{}, fmt.Errorf("numbers must be non-decreasing: %d < %d", n, prev)
		}
		af.Ranges = append(af.Ranges, NewSingleValueIntRange(n))
		prev = n
	}

	return af, nil
}

func NewAllNaturalRangeFilter() NaturalRangeFilter {
	return NaturalRangeFilter{
		Ranges: []IntRange{
			NewGreaterOrEqualThanIntRange(0),
		},
	}
}

// parseNaturalNumber parses s as an natural number and returns that value or an error.
// It returns an error if s is not a valid natural number representation.
func parseNaturalNumber(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("not natural number: %q", s)
	}
	return n, nil
}

// Test reports whether n is accepted by the filter.
func (f NaturalRangeFilter) Test(n int) bool {
	if n < 0 {
		return false
	}
	for _, r := range f.Ranges {
		if r.Contains(n) {
			return true
		}
	}
	return false
}

// IsNotEmpty reports whether the filter is not empty.
// It returns false if there are no ranges or all range is invalid.
func (f NaturalRangeFilter) IsNotEmpty() bool {
	if len(f.Ranges) == 0 {
		return false
	}
	greaterOrEqualThanZero := NewGreaterOrEqualThanIntRange(0)
	for _, r := range f.Ranges {
		if r.HasIntesect(greaterOrEqualThanZero) {
			return true
		}
	}
	return false
}

// IsAllNatural reports whether the filter accepts all natural numbers (0, 1, 2, ...).
func (f NaturalRangeFilter) IsAllNatural() bool {
	if len(f.Ranges) == 0 {
		return false
	}
	leftRangesCollection := [][]IntRange{
		{NewGreaterOrEqualThanIntRange(0)},
		{},
	}
	leftRangesIndex := 0
	for _, r := range f.Ranges {
		srcCollection := &leftRangesCollection[leftRangesIndex]
		tgtCollection := &leftRangesCollection[leftRangesIndex^1]
		*tgtCollection = (*tgtCollection)[:0]
		for _, leftRange := range *srcCollection {
			newLeftRanges := leftRange.Substract(r)
			for _, newLeftRange := range newLeftRanges {
				if newLeftRange.IsValid() {
					*tgtCollection = append(*tgtCollection, newLeftRange)
				}
			}
		}
		if len(*tgtCollection) == 0 {
			return true
		}
		leftRangesIndex ^= 1
	}
	return false
}

// Normalize 清理并归一化当前过滤器，返回一组互不重叠、按序且尽量简化的 NaturalRangeFilter
// 步骤：
//  1. 丢弃所有无效区间（!IsValid）
//  2. 若任意区间完全无界 -> 返回单个 NewUnboundedIntRange()
//  3. 将所有左无界的区间合并为一个 (-∞, M] 形式（M 为这些区间的最大有效上界）
//     将所有右无界的区间合并为一个 [N, +∞) 形式（N 为这些区间的最小有效下界）
//  4. 其余有界区间先转换为闭整型区间 [a,b]（根据包含/排除端点计算有效整数边界），丢弃空区间
//  5. 将所有区间按左端从小到大排序并合并所有能合并或相邻的区间（整数相邻也合并）
//  6. 返回转换回 IntRange 的结果（优先使用 -M / N- / N 或 N-M 表示）
func (f NaturalRangeFilter) Normalize() []IntRange {
	var valids []IntRange
	// 1) 过滤无效
	for _, r := range f.Ranges {
		n := r.TryMyBestToClosedInterval()
		if n.MinUnbounded || n.Min < 0 {
			n.MinUnbounded = false
			n.MinInclude = true
			n.Min = 0
		}
		if n.IsValid() && (n.MaxUnbounded || n.Max >= 0) {
			valids = append(valids, n)
		}
	}
	if len(valids) == 0 {
		return nil
	}

	// 简化表示为内部的简单区间结构（支持无穷端标记）
	type sRange struct {
		min   int
		max   int
		maxUn bool // max unbounded (+inf)
	}
	var srs []sRange

	// 2) 合并右无界集合（先计算合并边界），左边的界限已经提到0了，所以不可能是完全无界
	var rightMin int
	rightMinSet := false
	for _, r := range valids {
		if r.MaxUnbounded {
			if !rightMinSet || r.Min < rightMin {
				rightMin = r.Min
				rightMinSet = true
			}
		}
	}
	if rightMinSet {
		srs = append(srs, sRange{min: rightMin, maxUn: true})
	}

	// 3) 其余有界区间 -> 闭区间整数表示
	for _, r := range valids {
		if r.MinUnbounded || r.MaxUnbounded {
			continue
		}
		srs = append(srs, sRange{min: r.Min, max: r.Max})
	}

	if len(srs) == 0 {
		return nil
	}

	// 4) 排序（minUn true 的排在最前）
	sort.Slice(srs, func(i, j int) bool {
		a, b := srs[i], srs[j]
		if a.min != b.min {
			return a.min < b.min
		}
		// 如果左端相等，则把无穷右端排后（确保稳定性）
		if a.maxUn != b.maxUn {
			return !a.maxUn && b.maxUn
		}
		return a.max < b.max
	})

	// 合并：若相邻或重叠则合并；无穷边处理合并逻辑
	merged := make([]sRange, 0, len(srs))
	for _, cur := range srs {
		if len(merged) == 0 {
			merged = append(merged, cur)
			continue
		}
		last := merged[len(merged)-1]

		// 若 last 已经无穷右端，则直接跳过后续检查，因为它已经覆盖所有后续区间
		if last.maxUn {
			break
		}

		// 有界：若 last.max + 1 >= cur.min 则合并（整数相邻也合并）
		if last.max + 1 >= cur.min {
			last.max = cur.max
			merged[len(merged)-1] = last
			continue
		}

		// 否则不合并
		merged = append(merged, cur)
	}

	// 5) 转回 IntRange 列表（并保持升序）
	var out []IntRange
	for _, m := range merged {
		if m.maxUn {
			// [m, +inf) -> >= m
			out = append(out, NewGreaterOrEqualThanIntRange(m.min))
		} else if m.min == m.max {
			out = append(out, NewSingleValueIntRange(m.min))
		} else {
			out = append(out, NewInclusiveIntRange(m.min, m.max))
		}
	}

	return out
}

// String 先 Normalize 再生成尽可能可被 NewArgFilter 逆解析的字符串（数字非降序）。
// 规则输出： "all" / "-M" / "N-" / "N" / "N-M"，用 '_' 连接。
func (f NaturalRangeFilter) String() string {
	norm := f.Normalize()
	if len(norm) == 0 {
		return ""
	}
	// 若唯一且无界 -> "all"
	if len(norm) == 1 && norm[0].MaxUnbounded && norm[0].Min == 0 {
		return "all"
	}
	var parts []string
	for _, r := range norm {
		// 对每个归一化区间输出 NewArgFilter 能解析的形式
		if !r.IsUpperBounded() { // right unbounded -> "N-"
			parts = append(parts, fmt.Sprintf("%d-", r.Min))
			continue
		}
		// 有界
		if r.Min == r.Max {
			parts = append(parts, strconv.Itoa(r.Min))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", r.Min, r.Max))
		}
	}
	return strings.Join(parts, "_")
}
