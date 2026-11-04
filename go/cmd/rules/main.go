// Command rules serves deterministic eligibility and bundle templates from
// catalogue.product_catalogue (loaded by `make seed`). No LLM involvement.
//
//	GET  /products      the typed catalogue
//	POST /eligibility   {profile} -> eligible products and, per excluded product, reason codes
//	POST /bundles       {profile} -> segment bundle templates built from eligible products
package main

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"udyam/pkg/catalogue"
	"udyam/pkg/svc"
)

// products reads the catalogue on every call: 25 rows, and a re-seed is picked up immediately.
func products(ctx context.Context, db *pgxpool.Pool) ([]catalogue.Product, error) {
	rows, err := db.Query(ctx, `SELECT * FROM product_catalogue ORDER BY product_id`, pgx.QueryExecModeSimpleProtocol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var header []string
	for _, f := range rows.FieldDescriptions() {
		header = append(header, f.Name)
	}
	var data [][]string
	for rows.Next() {
		var rec []string
		for _, v := range rows.RawValues() {
			rec = append(rec, string(v))
		}
		data = append(data, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return catalogue.Parse(header, data)
}

type profileBody struct {
	Profile catalogue.Profile `json:"profile"`
}

func main() {
	svc.Run(svc.Options{
		Name: "rules", DefaultPort: "8082", Schema: "catalogue",
		Setup: func(ctx context.Context, a *svc.App) error {
			load := func(w http.ResponseWriter, r *http.Request) ([]catalogue.Product, bool) {
				ps, err := products(r.Context(), a.DB)
				if err != nil || len(ps) == 0 {
					a.Log.Error("catalogue load failed (run make seed?)", "err", err)
					svc.JSON(w, 503, map[string]string{"code": "CATALOGUE_UNAVAILABLE", "message": "catalogue not loaded"})
					return nil, false
				}
				return ps, true
			}
			withProfile := func(fn func(catalogue.Profile, []catalogue.Product) any) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					var body profileBody
					if !svc.DecodeJSON(w, r, &body) {
						return
					}
					if err := body.Profile.Validate(); err != nil {
						svc.JSON(w, 400, map[string]string{"code": "INVALID_REQUEST", "message": err.Error()})
						return
					}
					ps, ok := load(w, r)
					if !ok {
						return
					}
					svc.JSON(w, 200, fn(body.Profile, ps))
				}
			}

			a.Mux.HandleFunc("GET /products", func(w http.ResponseWriter, r *http.Request) {
				if ps, ok := load(w, r); ok {
					svc.JSON(w, 200, ps)
				}
			})
			a.Mux.HandleFunc("POST /eligibility", withProfile(func(p catalogue.Profile, ps []catalogue.Product) any {
				return catalogue.Evaluate(p, ps)
			}))
			a.Mux.HandleFunc("POST /bundles", withProfile(func(p catalogue.Profile, ps []catalogue.Product) any {
				return map[string]any{"bundles": catalogue.Bundles(p, catalogue.Evaluate(p, ps))}
			}))
			return nil
		},
	})
}
