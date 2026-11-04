package main

import "strings"

const gstinAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// GSTINChecksumValid checks the standard mod-36 check character (15th char):
// weights alternate 1,2; each product contributes quotient + remainder of /36.
func GSTINChecksumValid(g string) bool {
	if len(g) != 15 {
		return false
	}
	sum := 0
	for i := 0; i < 14; i++ {
		v := strings.IndexByte(gstinAlphabet, g[i])
		if v < 0 {
			return false
		}
		p := v * (1 + i%2)
		sum += p/36 + p%36
	}
	return gstinAlphabet[(36-sum%36)%36] == g[14]
}

// confusable maps characters OCR mixes up onto one representative: O/0, I/1/J/T, S/5, B/8.
func confusable(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case 'O':
			return '0'
		case 'I', 'J', 'T':
			return '1'
		case 'S':
			return '5'
		case 'B':
			return '8'
		}
		return r
	}, s)
}

func clean(id string) string {
	return strings.ToUpper(strings.Join(strings.Fields(id), ""))
}

// Index resolves IDs against one registry: exact, then OCR-confusion corrected, else none
// with near-miss suggestions (one differing character, e.g. a mistyped digit).
type Index[T any] struct {
	rows   map[string]T
	byConf map[string][]string
}

func NewIndex[T any](rows map[string]T) *Index[T] {
	ix := &Index[T]{rows: rows, byConf: map[string][]string{}}
	for id := range rows {
		k := confusable(id)
		ix.byConf[k] = append(ix.byConf[k], id)
	}
	return ix
}

type Match[T any] struct {
	Query       string   `json:"query"`
	MatchType   string   `json:"matchType"` // exact | corrected | none
	ID          string   `json:"id,omitempty"`
	Record      *T       `json:"record,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

func (ix *Index[T]) Resolve(query string) Match[T] {
	q := clean(query)
	m := Match[T]{Query: q, MatchType: "none"}
	if r, ok := ix.rows[q]; ok {
		m.MatchType, m.ID, m.Record = "exact", q, &r
		return m
	}
	if c := ix.byConf[confusable(q)]; len(c) == 1 {
		r := ix.rows[c[0]]
		m.MatchType, m.ID, m.Record = "corrected", c[0], &r
		return m
	} else if len(c) > 1 {
		m.Suggestions = c // ambiguous correction: let a human pick
		return m
	}
	// ponytail: linear scan over ~3.5k ids per miss; index by deletion keys if registries grow.
	cq := confusable(q)
	for k, ids := range ix.byConf {
		if len(k) == len(cq) && hamming(k, cq) == 1 {
			m.Suggestions = append(m.Suggestions, ids...)
		}
	}
	return m
}

func hamming(a, b string) int {
	d := 0
	for i := range a {
		if a[i] != b[i] {
			d++
		}
	}
	return d
}
