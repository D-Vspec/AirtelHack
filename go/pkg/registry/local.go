package registry

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Local serves lookups from in-memory indexes. Tests and offline tools build it from the case
// CSVs; the registry service builds the same indexes from Postgres.
type Local struct {
	G *Index[GSTIN]
	U *Index[Udyam]
}

func (l *Local) GSTIN(_ context.Context, id string) (GSTINMatch, error) {
	m := l.G.Resolve(id)
	sort.Strings(m.Suggestions)
	return GSTINMatch{Match: m, ChecksumValid: GSTINChecksumValid(m.Query)}, nil
}

func (l *Local) Udyam(_ context.Context, id string) (Match[Udyam], error) {
	m := l.U.Resolve(id)
	sort.Strings(m.Suggestions)
	return m, nil
}

// LoadCSV reads mock_gstin_registry.csv and mock_udyam_registry.csv from dataDir.
func LoadCSV(dataDir string) (*Local, error) {
	read := func(name string, cols int) ([][]string, error) {
		f, err := os.Open(filepath.Join(dataDir, name))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		rows, err := csv.NewReader(f).ReadAll()
		if err != nil {
			return nil, err
		}
		if len(rows) < 1 || len(rows[0]) != cols {
			return nil, fmt.Errorf("%s: expected %d columns", name, cols)
		}
		return rows[1:], nil
	}
	gr, err := read("mock_gstin_registry.csv", 8)
	if err != nil {
		return nil, err
	}
	ur, err := read("mock_udyam_registry.csv", 8)
	if err != nil {
		return nil, err
	}
	g := make(map[string]GSTIN, len(gr))
	for _, r := range gr {
		g[r[0]] = GSTIN{GSTIN: r[0], LegalName: r[1], TradeName: r[2], Constitution: r[3], Address: r[4], State: r[5], RegistrationDate: r[6], Status: r[7]}
	}
	u := make(map[string]Udyam, len(ur))
	for _, r := range ur {
		u[r[0]] = Udyam{UdyamNumber: r[0], EnterpriseName: r[1], EnterpriseType: r[2], MajorActivity: r[3], NICDescription: r[4], State: r[5], City: r[6], DateOfRegistration: r[7]}
	}
	return &Local{G: NewIndex(g), U: NewIndex(u)}, nil
}
