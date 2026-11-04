// Command simulator stands in for installation ops in the demo. Every tick it moves orders one
// step along the happy path (kyc_passed -> install_scheduled -> installing -> activated) through
// the tracker's HTTP API, once an order has sat in its status for SIM_STEP_SECONDS. Orders whose
// lead ID contains "nofibre" hit the fibre_not_feasible branch instead, which makes the tracker
// offer 5G AirFiber; they wait there until the owner accepts the offer.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"udyam/pkg/config"
	"udyam/pkg/logx"
	"udyam/pkg/orders"
)

type order struct {
	ID              string    `json:"id"`
	LeadID          string    `json:"leadId"`
	Status          string    `json:"status"`
	ProductIDs      []string  `json:"productIds"`
	FallbackOffered bool      `json:"fallbackOffered"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// nextMove decides the simulator's move for one order, or "" to leave it.
func nextMove(o order, now time.Time, step time.Duration) (to, reason string) {
	if now.Sub(o.UpdatedAt) < step {
		return "", ""
	}
	switch o.Status {
	case orders.KycPassed:
		if strings.Contains(o.LeadID, "nofibre") && len(orders.FibreIn(o.ProductIDs)) > 0 && !o.FallbackOffered {
			return orders.Delayed, orders.ReasonFibreNotFeasible
		}
		return orders.InstallScheduled, "simulator"
	case orders.InstallScheduled, orders.Installing:
		return orders.Next(o.Status), "simulator"
	}
	return "", "" // kyc_pending waits for KYC, delayed waits for the owner, final states stay
}

func main() {
	log := logx.New("simulator")
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	base := config.String("TRACKER_URL", "http://tracker:8085")
	step := time.Duration(config.Int("SIM_STEP_SECONDS", 8)) * time.Second
	tick := time.Duration(config.Int("SIM_TICK_SECONDS", 2)) * time.Second
	client := &http.Client{Timeout: 5 * time.Second}
	log.Info("simulator started", "tracker", base, "stepSeconds", step.Seconds())
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		if err := round(ctx, client, base, step, log.Info); err != nil && ctx.Err() == nil {
			log.Warn("simulator round failed", "err", err)
		}
		select {
		case <-ctx.Done():
			log.Info("stopped")
			os.Exit(0)
		case <-t.C:
		}
	}
}

func round(ctx context.Context, c *http.Client, base string, step time.Duration, info func(string, ...any)) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/orders?status=kyc_passed,install_scheduled,installing", nil)
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	var body struct {
		Orders []order `json:"orders"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	now := time.Now()
	for _, o := range body.Orders {
		to, reason := nextMove(o, now, step)
		if to == "" {
			continue
		}
		b, _ := json.Marshal(map[string]string{"to": to, "reason": reason})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/orders/"+o.ID+"/transitions", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != 200 && resp.StatusCode != 409 { // 409: someone moved it first
			return fmt.Errorf("transition %s -> %s: status %d", o.ID, to, resp.StatusCode)
		}
		info("order advanced", "orderId", o.ID, "leadId", o.LeadID, "from", o.Status, "to", to)
	}
	return nil
}
