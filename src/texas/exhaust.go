package texas

func getAllCards() map[int8]int {
	ret := make(map[int8]int)
	for _, p := range allCards {
		ret[p] = 1
	}
	return ret
}

func tolist(m map[int8]int) []int8 {
	var ret []int8
	for k, _ := range m {
		ret = append(ret, k)
	}
	return ret
}

func permutation(f func(tmp []int8), a []int8, count int, count2 int, except int, tmp []int8) {
	if count2 == except {
		f(tmp)
	} else {
		for i := count; i < len(a); i++ {
			tmp[count2] = a[i]
			permutation(f, a, i+1, count2+1, except, tmp)
		}
	}
}

func GetExHandProbability(hand string, pub string) float32 {
	return getExHandProbabilityByBytes(StrToBytes(hand), StrToBytes(pub))
}

type calcData struct {
	win   int64
	lose  int64
	tie   int64
	total int64
}

func getExHandProbabilityByBytes(hand []int8, pub []int8) float32 {
	list := getAllCards()

	for _, p := range hand {
		delete(list, p)
	}
	for _, p := range pub {
		delete(list, p)
	}

	a := tolist(list)
	otherhand := make([]int8, 2)

	data := &calcData{}

	permutation(func(tmp []int8) {
		onOtherHandGen(hand, pub, tmp, data, &list, calcExHandProbabilityByBytes)
	}, a, 0, 0, 2, otherhand)

	return (float32(data.win) + float32(data.tie)*0.5) / float32(data.total)
}

func onOtherHandGen(hand []int8, pub []int8, otherhand []int8, data *calcData, list *map[int8]int,
	f func(hand []int8, pub []int8, otherhand []int8, leftpub []int8, data *calcData)) {

	for _, p := range otherhand {
		delete(*list, p)
	}

	a := tolist(*list)
	leftpub := make([]int8, 5-len(pub))

	permutation(func(tmp []int8) {
		onLeftPubGen(hand, pub, otherhand, tmp, data, list, f)
	}, a, 0, 0, 5-len(pub), leftpub)

	for _, p := range otherhand {
		(*list)[p] = 1
	}
}

func onLeftPubGen(hand []int8, pub []int8, otherhand []int8, leftpub []int8, data *calcData, list *map[int8]int,
	f func(hand []int8, pub []int8, otherhand []int8, leftpub []int8, data *calcData)) {
	f(hand, pub, otherhand, leftpub, data)
}

func calcExHandProbabilityByBytes(hand []int8, pub []int8, otherhand []int8, leftpub []int8, data *calcData) {

	var my []int8
	var other []int8

	my = append(my, hand...)
	my = append(my, pub...)
	my = append(my, leftpub...)

	other = append(other, otherhand...)
	other = append(other, pub...)
	other = append(other, leftpub...)

	ret := CompareByBytes(my, other)
	if ret == 0 {
		data.tie++
	} else if ret < 0 {
		data.lose++
	} else if ret > 0 {
		data.win++
	}
	data.total++
}

func GetExTypeProbability(hand string, pub string, t string) float32 {
	return GetExTypeProbabilityByBytes(StrToBytes(hand), StrToBytes(pub), GetWinNameId(t))
}

func GetExTypeProbabilityByBytes(hand []int8, pub []int8, t int) float32 {
	list := getAllCards()

	for _, p := range hand {
		delete(list, p)
	}
	for _, p := range pub {
		delete(list, p)
	}

	a := tolist(list)
	otherhand := make([]int8, 2)

	data := &calcData{}

	permutation(func(tmp []int8) {
		onOtherHandGen(hand, pub, tmp, data, &list, func(hand []int8, pub []int8, otherhand []int8, leftpub []int8, data *calcData) {

			var my []int8
			my = append(my, hand...)
			my = append(my, pub...)
			my = append(my, leftpub...)

			ret := GetWinTypeByBytes(my)
			if ret == t {
				data.win++
			}
			data.total++
		})
	}, a, 0, 0, 2, otherhand)

	return float32(data.win) / float32(data.total)
}

func GetExAllTypeProbability(hand string, pub string) []float32 {
	return GetExAllTypeProbabilityByBytes(StrToBytes(hand), StrToBytes(pub))
}

func GetExAllTypeProbabilityByBytes(hand []int8, pub []int8) []float32 {
	list := getAllCards()

	for _, p := range hand {
		delete(list, p)
	}
	for _, p := range pub {
		delete(list, p)
	}

	a := tolist(list)
	otherhand := make([]int8, 2)

	data := &calcData{}
	var retp []float32
	retp = make([]float32, len(winName))

	permutation(func(tmp []int8) {
		onOtherHandGen(hand, pub, tmp, data, &list, func(hand []int8, pub []int8, otherhand []int8, leftpub []int8, data *calcData) {

			var my []int8
			my = append(my, hand...)
			my = append(my, pub...)
			my = append(my, leftpub...)

			ret := GetWinTypeByBytes(my)
			retp[ret]++
			data.total++
		})
	}, a, 0, 0, 2, otherhand)

	for i, p := range retp {
		retp[i] = p / float32(data.total)
	}

	return retp
}
