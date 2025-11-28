package bind

// Ordered 限定为可比较的数值类型（整型/无符号/浮点）
// 使用 ~ 前缀以允许基于这些底层类型的自定义类型也满足约束。
type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

type Range[T Ordered] interface {

	// Contains 判断 v 是否在区间内（闭/开取决具体实现）
	Contains(v T) bool
	IsNotEmpty() bool
	IsLowerBounded() bool
	IsUpperBounded() bool
}
