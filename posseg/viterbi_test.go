package posseg

import (
	"testing"
)

var (
	route1 = []string{
		"Bnr",
		"Mnr",
		"Enr",
		"Sv",
		"Bv",
		"Ev",
		"Bn",
		"Mn",
		"En",
		"Sd",
		"Sv",
		"Sn",
		"Bv",
		"Ev",
		"Bnr",
		"Mnr",
		"Mnr",
		"Mnr",
		"Enr",
		"Szg"}
)

func TestViterbi(t *testing.T) {
	ss := "李小福是创新办主任也是云计算方面的专家;"
	route := viterbi([]rune(ss))
	if len(route) != len(route1) {
		t.Error(len(route))
	}
	for index, _ := range route {
		if route[index] != route1[index] {
			t.Error(route[index])
		}
	}
}
