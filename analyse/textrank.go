package analyse

import (
	"fmt"
	"github.com/wangbin/jiebago/posseg"
	"math"
	"sort"
)

const dampingFactor = 0.85

var (
	defaultAllowPOS = []string{"ns", "n", "vn", "v"}
)

type edge struct {
	start  string
	end    string
	weight float64
}

func (e edge) String() string {
	return fmt.Sprintf("(%s %s): %f", e.start, e.end, e.weight)
}

type edges []edge

func (es edges) Len() int {
	return len(es)
}

func (es edges) Less(i, j int) bool {
	return es[i].weight < es[j].weight
}

func (es edges) Swap(i, j int) {
	es[i], es[j] = es[j], es[i]
}

type undirectWeightedGraph struct {
	graph map[string]edges
	keys  sort.StringSlice
}

func newUndirectWeightedGraph() *undirectWeightedGraph {
	u := new(undirectWeightedGraph)
	u.graph = make(map[string]edges)
	u.keys = make(sort.StringSlice, 0)
	return u
}

func (u *undirectWeightedGraph) addEdge(start, end string, weight float64) {
	if _, ok := u.graph[start]; !ok {
		u.keys = append(u.keys, start)
		u.graph[start] = edges{edge{start: start, end: end, weight: weight}}
	} else {
		u.graph[start] = append(u.graph[start], edge{start: start, end: end, weight: weight})
	}

	if _, ok := u.graph[end]; !ok {
		u.keys = append(u.keys, end)
		u.graph[end] = edges{edge{start: end, end: start, weight: weight}}
	} else {
		u.graph[end] = append(u.graph[end], edge{start: end, end: start, weight: weight})
	}
}

func (u *undirectWeightedGraph) rank() wordWeights {
	if !sort.IsSorted(u.keys) {
		sort.Sort(u.keys)
	}

	ws := make(map[string]float64)
	outSum := make(map[string]float64)

	wsdef := 1.0
	if len(u.graph) > 0 {
		wsdef /= float64(len(u.graph))
	}
	for n, out := range u.graph {
		ws[n] = wsdef
		sum := 0.0
		for _, e := range out {
			sum += e.weight
		}
		outSum[n] = sum
	}

	for x := 0; x < 10; x++ {
		for _, n := range u.keys {
			s := 0.0
			inedges := u.graph[n]
			for _, e := range inedges {
				s += e.weight / outSum[e.end] * ws[e.end]
			}
			ws[n] = (1 - dampingFactor) + dampingFactor*s
		}
	}
	minRank := math.MaxFloat64
	maxRank := math.SmallestNonzeroFloat64
	for _, w := range ws {
		if w < minRank {
			minRank = w
		} else if w > maxRank {
			maxRank = w
		}
	}
	result := make(wordWeights, 0)
	for n, w := range ws {
		result = append(result, wordWeight{Word: n, Weight: (w - minRank/10.0) / (maxRank - minRank/10.0)})
	}
	sort.Sort(sort.Reverse(result))
	return result
}

// Extract keywords from sentence using TextRank algorithm. the allowed POS list
// could be manually speificed.
func (t *TextRanker) TextRankWithPOS(sentence string, topK int, allowPOS []string) wordWeights {
	posFilt := make(map[string]int)
	for _, pos := range allowPOS {
		posFilt[pos] = 1
	}
	g := newUndirectWeightedGraph()
	cm := make(map[[2]string]float64)
	span := 5
	pairs := make([]posseg.Pair, 0)
	for pair := range t.Cut(sentence, true) {
		pairs = append(pairs, pair)
	}
	for i, _ := range pairs {
		if _, ok := posFilt[pairs[i].Flag]; ok {
			for j := i + 1; j < i+span && j <= len(pairs); j++ {
				if _, ok := posFilt[pairs[j].Flag]; !ok {
					continue
				}
				if _, ok := cm[[2]string{pairs[i].Word, pairs[j].Word}]; !ok {
					cm[[2]string{pairs[i].Word, pairs[j].Word}] = 1.0
				} else {
					cm[[2]string{pairs[i].Word, pairs[j].Word}] += 1.0
				}
			}
		}
	}
	for startEnd, weight := range cm {
		g.addEdge(startEnd[0], startEnd[1], weight)
	}
	tags := g.rank()
	if topK > 0 && len(tags) > topK {
		tags = tags[:topK]
	}
	return tags
}

// Extract keywords from sentence using TextRank algorithm.
// topK specify how many top keywords to be returned at most.
func (t *TextRanker) TextRank(sentence string, topK int) wordWeights {
	return t.TextRankWithPOS(sentence, topK, defaultAllowPOS)
}

// Set the dictionary, could be absolute path of dictionary file, or dictionary
// name in current directory. This function must be called before cut any
// sentence.
func NewTextRanker(dictFileName string) (*TextRanker, error) {
	p, err := posseg.NewPosseg(dictFileName)
	if err != nil {
		return nil, err
	}
	return &TextRanker{p}, nil
}

type TextRanker struct {
	*posseg.Posseg
}
