package common

import "hash/fnv"

func MinOfInt(vars ...int) int {
	min := vars[0]

	for _, i := range vars {
		if min > i {
			min = i
		}
	}

	return min
}

func MaxOfInt(vars ...int) int {
	max := vars[0]

	for _, i := range vars {
		if max < i {
			max = i
		}
	}

	return max
}

func MinOfInt64(vars ...int64) int64 {
	min := vars[0]

	for _, i := range vars {
		if min > i {
			min = i
		}
	}

	return min
}

func MaxOfInt64(vars ...int64) int64 {
	max := vars[0]

	for _, i := range vars {
		if max < i {
			max = i
		}
	}

	return max
}

func AbsInt(v int) int {
	if v > 0 {
		return v
	}
	return -v
}

func AbsInt32(v int32) int32 {
	if v > 0 {
		return v
	}
	return -v
}

func AbsInt64(v int64) int64 {
	if v > 0 {
		return v
	}
	return -v
}

func HashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
