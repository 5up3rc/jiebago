package posseg

import (
	"fmt"
	"github.com/wangbin/jiebago"
	"regexp"
	"strings"
)

var (
	reHanDetail    = regexp.MustCompile(`(\p{Han}+)`)
	reSkipDetail   = regexp.MustCompile(`([[\.[:digit:]]+|[:alnum:]]+)`)
	reEng          = regexp.MustCompile(`[[:alnum:]]`)
	reNum          = regexp.MustCompile(`[\.[:digit:]]+`)
	reEng1         = regexp.MustCompile(`[[:alnum:]]$`)
	reHanInternal  = regexp.MustCompile(`([\p{Han}+[:alnum:]+#&\._]+)`)
	reSkipInternal = regexp.MustCompile(`(\r\n|\s)`)
)

type Pair struct {
	Word, Flag string
}

func (p Pair) String() string {
	return fmt.Sprintf("%s / %s", p.Word, p.Flag)
}

type Posseg struct {
	*jiebago.Jieba
	flagMap map[string]string
}

func (p *Posseg) AddEntry(entry jiebago.Entry) {
	if len(entry.Flag) > 0 {
		p.flagMap[entry.Word] = strings.TrimSpace(entry.Flag)
	}
	p.Add(entry.Word, entry.Freq)
}

func (p Posseg) Flag(word string) (string, bool) {
	flag, ok := p.flagMap[word]
	return flag, ok
}

// Set dictionary, it could be absolute path of dictionary file, or dictionary
// name in current diectory.
func Open(dictFileName string) (*Posseg, error) {
	p := New()
	err := jiebago.LoadDict(p, dictFileName, true)
	return p, err
}

// Load user specified dictionary file.
func (p *Posseg) LoadUserDict(dictFileName string) error {
	return jiebago.LoadDict(p, dictFileName, true)
}

func (p *Posseg) SetDict(dictFileName string) error {
	if len(p.flagMap) > 0 || p.Total() > 0.0 {
		return jiebago.ErrInitialized
	}
	return jiebago.LoadDict(p, dictFileName, false)
}

func New() *Posseg {
	return &Posseg{jiebago.New(), make(map[string]string)}
}

func (p *Posseg) cutDetailInternal(sentence string) chan Pair {
	result := make(chan Pair)

	go func() {
		runes := []rune(sentence)
		posList := viterbi(runes)
		begin := 0
		next := 0
		for i, char := range runes {
			pos := posList[i]
			switch pos.Tag() {
			case "B":
				begin = i
			case "E":
				result <- Pair{string(runes[begin : i+1]), pos.POS()}
				next = i + 1
			case "S":
				result <- Pair{string(char), pos.POS()}
				next = i + 1
			}
		}
		if next < len(runes) {
			result <- Pair{string(runes[next:]), posList[next].POS()}
		}
		close(result)
	}()
	return result
}

func (p *Posseg) cutDetail(sentence string) chan Pair {
	result := make(chan Pair)
	go func() {
		for _, blk := range jiebago.RegexpSplit(reHanDetail, sentence, -1) {
			if reHanDetail.MatchString(blk) {
				for wordTag := range p.cutDetailInternal(blk) {
					result <- wordTag
				}
			} else {
				for _, x := range jiebago.RegexpSplit(reSkipDetail, blk, -1) {
					if len(x) == 0 {
						continue
					}
					switch {
					case reNum.MatchString(x):
						result <- Pair{x, "m"}
					case reEng.MatchString(x):
						result <- Pair{x, "eng"}
					default:
						result <- Pair{x, "x"}
					}
				}
			}
		}
		close(result)
	}()
	return result
}

type cutFunc func(sentence string) chan Pair

func (p *Posseg) cutDAG(sentence string) chan Pair {
	result := make(chan Pair)

	go func() {
		runes := []rune(sentence)
		dag := jiebago.DAG(p, runes)
		routes := jiebago.Routes(p, runes, dag)
		var y int
		length := len(runes)
		buf := make([]rune, 0)
		for x := 0; x < length; {
			y = routes[x].Index + 1
			l_word := runes[x:y]
			if y-x == 1 {
				buf = append(buf, l_word...)
			} else {
				if len(buf) > 0 {
					if len(buf) == 1 {
						sbuf := string(buf)
						if tag, ok := p.Flag(sbuf); ok {
							result <- Pair{sbuf, tag}
						} else {
							result <- Pair{sbuf, "x"}
						}
						buf = make([]rune, 0)
					} else {
						bufString := string(buf)
						if v, ok := p.Freq(bufString); !ok || v == 0.0 {
							for t := range p.cutDetail(bufString) {
								result <- t
							}
						} else {
							for _, elem := range buf {
								selem := string(elem)
								if tag, ok := p.Flag(selem); ok {
									result <- Pair{string(elem), tag}
								} else {
									result <- Pair{string(elem), "x"}
								}

							}
						}
						buf = make([]rune, 0)
					}
				}
				sl_word := string(l_word)
				if tag, ok := p.Flag(sl_word); ok {
					result <- Pair{sl_word, tag}
				} else {
					result <- Pair{sl_word, "x"}
				}
			}
			x = y
		}

		if len(buf) > 0 {
			if len(buf) == 1 {
				sbuf := string(buf)
				if tag, ok := p.Flag(sbuf); ok {
					result <- Pair{sbuf, tag}
				} else {
					result <- Pair{sbuf, "x"}
				}
			} else {
				bufString := string(buf)
				if v, ok := p.Freq(bufString); !ok || v == 0.0 {
					for t := range p.cutDetail(bufString) {
						result <- t
					}
				} else {
					for _, elem := range buf {
						selem := string(elem)
						if tag, ok := p.Flag(selem); ok {
							result <- Pair{selem, tag}
						} else {
							result <- Pair{selem, "x"}
						}
					}
				}
			}
		}
		close(result)
	}()
	return result
}

func (p *Posseg) cutDAGNoHMM(sentence string) chan Pair {
	result := make(chan Pair)

	go func() {
		runes := []rune(sentence)
		dag := jiebago.DAG(p, runes)
		routes := jiebago.Routes(p, runes, dag)
		x := 0
		var y int
		length := len(runes)
		buf := make([]rune, 0)
		for {
			if x >= length {
				break
			}
			y = routes[x].Index + 1
			l_word := runes[x:y]
			if reEng1.MatchString(string(l_word)) && len(l_word) == 1 {
				buf = append(buf, l_word...)
				x = y
			} else {
				if len(buf) > 0 {
					result <- Pair{string(buf), "eng"}
					buf = make([]rune, 0)
				}
				sl_word := string(l_word)
				if tag, ok := p.Flag(sl_word); ok {
					result <- Pair{sl_word, tag}
				} else {
					result <- Pair{sl_word, "x"}
				}
				x = y
			}
		}
		if len(buf) > 0 {
			result <- Pair{string(buf), "eng"}
			buf = make([]rune, 0)
		}
		close(result)
	}()
	return result
}

// Tags the POS of each word after segmentation, using labels compatible with
// ictclas.
func (p *Posseg) Cut(sentence string, HMM bool) chan Pair {
	result := make(chan Pair)
	var cut cutFunc
	if HMM {
		cut = p.cutDAG
	} else {
		cut = p.cutDAGNoHMM
	}
	go func() {
		for _, blk := range jiebago.RegexpSplit(reHanInternal, sentence, -1) {
			if reHanInternal.MatchString(blk) {
				for wordTag := range cut(blk) {
					result <- wordTag
				}
			} else {
				for _, x := range jiebago.RegexpSplit(reSkipInternal, blk, -1) {
					if reSkipInternal.MatchString(x) {
						result <- Pair{x, "x"}
					} else {
						for _, xx := range x {
							s := string(xx)
							switch {
							case reNum.MatchString(s):
								result <- Pair{s, "m"}
							case reEng.MatchString(x):
								result <- Pair{x, "eng"}
								break
							default:
								result <- Pair{s, "x"}
							}
						}
					}
				}
			}
		}
		close(result)
	}()
	return result
}
