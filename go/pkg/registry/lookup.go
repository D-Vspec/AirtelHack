// Package registry resolves GSTIN and Udyam numbers against the mock registries (exact, OCR-corrected, near-miss).
package registry

import (
	"encoding/csv"
	"os"
	"strings"
)

type GSTIN struct {
	GSTIN            string `json:"gstin" db:"gstin"`
	LegalName        string `json:"legalName" db:"legal_name"`
	TradeName        string `json:"tradeName" db:"trade_name"`
	Constitution     string `json:"constitution" db:"constitution"`
	Address          string `json:"address" db:"principal_place_of_business"`
	State            string `json:"state" db:"state"`
	RegistrationDate string `json:"registrationDate" db:"registration_date"`
	Status           string `json:"status" db:"status"`
}

type Udyam struct {
	UdyamNumber        string `json:"udyamNumber" db:"udyam_number"`
	EnterpriseName     string `json:"enterpriseName" db:"enterprise_name"`
	EnterpriseType     string `json:"enterpriseType" db:"enterprise_type"`
	MajorActivity      string `json:"majorActivity" db:"major_activity"`
	NICDescription     string `json:"nicDescription" db:"nic_description"`
	State              string `json:"state" db:"state"`
	City               string `json:"city" db:"city"`
	DateOfRegistration string `json:"dateOfRegistration" db:"date_of_registration"`
}

// LoadCSV reads the registry CSVs in data/ (tests and offline tools; the service loads from Postgres).
func LoadCSV(gstinPath, udyamPath string) (*Index[GSTIN], *Index[Udyam], error) {
	read := func(p string) ([][]string, error) {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		rows, err := csv.NewReader(f).ReadAll()
		if len(rows) > 0 {
			rows = rows[1:]
		}
		return rows, err
	}
	gr, err := read(gstinPath)
	if err != nil {
		return nil, nil, err
	}
	ur, err := read(udyamPath)
	if err != nil {
		return nil, nil, err
	}
	g, u := map[string]GSTIN{}, map[string]Udyam{}
	for _, r := range gr {
		g[r[0]] = GSTIN{r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7]}
	}
	for _, r := range ur {
		u[r[0]] = Udyam{r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7]}
	}
	return NewIndex(g), NewIndex(u), nil
}

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

// confusable maps characters OCR mixes up onto one representative: O/0, I/1/J/T, S/5, B/8, Z/7
// (tesseract 5.3 reads K05's Z as 7).
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
		case 'Z':
			return '7'
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
