package jiebago

import (
	"crypto/md5"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Trie store the total frequency and map of all words and their frequenciesb
var Trie *trie

type trie struct {
	Total float64
	Freq  map[string]float64
}

func (t *trie) load(dictFileName string) error {
	dictFilePath, err := DictPath(dictFileName)
	if err != nil {
		return err
	}

	dictFileInfo, err := os.Stat(dictFilePath)
	if err != nil {
		return err
	}

	log.Printf("Building Trie..., from %s\n", dictFilePath)
	h := fmt.Sprintf("%x", md5.Sum([]byte(dictFilePath)))
	cacheFileName := fmt.Sprintf("jieba.%s.cache", h)
	cacheFilePath := filepath.Join(os.TempDir(), cacheFileName)
	isDictCached := true

	cacheFileInfo, err := os.Stat(cacheFilePath)
	if err != nil {
		isDictCached = false
	}

	if isDictCached {
		isDictCached = cacheFileInfo.ModTime().After(dictFileInfo.ModTime())
	}

	var cacheFile *os.File
	if isDictCached {
		cacheFile, err = os.Open(cacheFilePath)
		if err != nil {
			isDictCached = false
		}
		defer cacheFile.Close()
	}

	if isDictCached {
		dec := gob.NewDecoder(cacheFile)
		err = dec.Decode(&t)
		if err != nil {
			isDictCached = false
		} else {
			log.Printf("loaded model from cache %s\n", cacheFilePath)
		}
	}

	if !isDictCached {
		wtfs, err := ParseDictFile(dictFilePath)
		if err != nil {
			return err
		}

		for _, wtf := range wtfs {
			t.addWord(wtf)
		}
		// dump trie
		cacheFile, err = os.OpenFile(cacheFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer cacheFile.Close()
		enc := gob.NewEncoder(cacheFile)
		err = enc.Encode(t)
		if err != nil {
			return err
		} else {
			log.Printf("dumped model from cache %s\n", cacheFilePath)
		}
	}
	return nil
}

func (t *trie) addWord(wtf *WordTagFreq) {
	t.Freq[wtf.Word] = wtf.Freq
	t.Total += wtf.Freq
	runes := []rune(wtf.Word)
	count := len(runes)
	for i := 0; i < count; i++ {
		wfrag := string(runes[0 : i+1])
		if _, ok := t.Freq[wfrag]; !ok {
			t.Freq[wfrag] = 0.0
		}
	}
}

// Load user specified dictionary file.
func LoadUserDict(dictFilePath string) error {
	wtfs, err := ParseDictFile(dictFilePath)
	if err != nil {
		return err
	}
	for _, wtf := range wtfs {
		if len(wtf.Tag) > 0 {
			UserWordTagTab[wtf.Word] = strings.TrimSpace(wtf.Tag)
		}
		Trie.addWord(wtf)
	}
	return nil
}

// Set the dictionary, could be absolute path of dictionary file, or dictionary
// name in current directory. This function must be called before cut any
// sentence.
func SetDictionary(dictFileName string) error {
	Trie = &trie{Total: 0.0, Freq: make(map[string]float64)}
	return Trie.load(dictFileName)
}
