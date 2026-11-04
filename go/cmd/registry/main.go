// Command registry serves GSTIN and Udyam lookups backed by registry.gstin / registry.udyam
// (loaded by `make seed`): GET /gstin/{id}, GET /udyam/{id}.
package main

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"udyam/pkg/logx"
	"udyam/pkg/svc"
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

// registries is loaded lazily so the service starts before `make seed` has run.
type registries struct {
	mu    sync.Mutex
	gstin *Index[GSTIN]
	udyam *Index[Udyam]
}

func load[T any](ctx context.Context, db *pgxpool.Pool, sql string, key func(T) string) (map[string]T, error) {
	rows, err := db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[T])
	if err != nil {
		return nil, err
	}
	m := make(map[string]T, len(list))
	for _, r := range list {
		m[key(r)] = r
	}
	return m, nil
}

func (r *registries) get(ctx context.Context, db *pgxpool.Pool) (*Index[GSTIN], *Index[Udyam], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gstin == nil {
		g, err := load(ctx, db, `SELECT gstin, legal_name, trade_name, constitution, principal_place_of_business, state,
			registration_date::text AS registration_date, status FROM gstin`, func(x GSTIN) string { return x.GSTIN })
		if err != nil {
			return nil, nil, err
		}
		u, err := load(ctx, db, `SELECT udyam_number, enterprise_name, enterprise_type, major_activity, nic_description, state, city,
			date_of_registration::text AS date_of_registration FROM udyam`, func(x Udyam) string { return x.UdyamNumber })
		if err != nil {
			return nil, nil, err
		}
		r.gstin, r.udyam = NewIndex(g), NewIndex(u)
	}
	return r.gstin, r.udyam, nil
}

func main() {
	reg := &registries{}
	svc.Run(svc.Options{
		Name: "registry", DefaultPort: "8081", Schema: "registry",
		Setup: func(ctx context.Context, a *svc.App) error {
			indexes := func(w http.ResponseWriter, r *http.Request) (*Index[GSTIN], *Index[Udyam], bool) {
				c, cancel := context.WithTimeout(r.Context(), 10*time.Second)
				defer cancel()
				g, u, err := reg.get(c, a.DB)
				if err != nil {
					a.Log.Error("registry load failed (run make seed?)", "err", err)
					svc.JSON(w, 503, map[string]string{"error": "registry_unavailable"})
					return nil, nil, false
				}
				return g, u, true
			}

			a.Mux.HandleFunc("GET /gstin/{id}", func(w http.ResponseWriter, r *http.Request) {
				g, _, ok := indexes(w, r)
				if !ok {
					return
				}
				m := g.Resolve(r.PathValue("id"))
				sort.Strings(m.Suggestions)
				a.Log.Info("gstin lookup", "gstin", logx.MaskID(m.Query), "matchType", m.MatchType)
				svc.JSON(w, status(m.MatchType), struct {
					Match[GSTIN]
					ChecksumValid bool `json:"checksumValid"`
				}{m, GSTINChecksumValid(m.Query)})
			})

			a.Mux.HandleFunc("GET /udyam/{id}", func(w http.ResponseWriter, r *http.Request) {
				_, u, ok := indexes(w, r)
				if !ok {
					return
				}
				m := u.Resolve(r.PathValue("id"))
				sort.Strings(m.Suggestions)
				a.Log.Info("udyam lookup", "udyam", logx.MaskID(m.Query), "matchType", m.MatchType)
				svc.JSON(w, status(m.MatchType), m)
			})
			return nil
		},
	})
}

func status(matchType string) int {
	if matchType == "none" {
		return http.StatusNotFound
	}
	return http.StatusOK
}
