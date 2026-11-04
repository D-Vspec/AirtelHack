package main

import (
	"testing"
	"time"

	"udyam/pkg/orders"
)

func TestNextMove(t *testing.T) {
	now := time.Now()
	old := now.Add(-time.Minute)
	cases := []struct {
		o      order
		to, rs string
	}{
		{order{Status: orders.KycPassed, UpdatedAt: now}, "", ""}, // too soon
		{order{Status: orders.KycPassed, UpdatedAt: old, ProductIDs: []string{"P01"}}, orders.InstallScheduled, "simulator"},
		{order{Status: orders.InstallScheduled, UpdatedAt: old}, orders.Installing, "simulator"},
		{order{Status: orders.Installing, UpdatedAt: old}, orders.Activated, "simulator"},
		{order{LeadID: "demo-nofibre-1", Status: orders.KycPassed, UpdatedAt: old, ProductIDs: []string{"P02", "P16"}}, orders.Delayed, orders.ReasonFibreNotFeasible},
		{order{LeadID: "demo-nofibre-2", Status: orders.KycPassed, UpdatedAt: old, ProductIDs: []string{"P07"}}, orders.InstallScheduled, "simulator"},
		{order{Status: orders.KycPending, UpdatedAt: old}, "", ""},
		{order{Status: orders.Delayed, UpdatedAt: old}, "", ""},
		{order{Status: orders.Activated, UpdatedAt: old}, "", ""},
	}
	for _, c := range cases {
		to, rs := nextMove(c.o, now, 8*time.Second)
		if to != c.to || rs != c.rs {
			t.Errorf("%+v: got %s/%s want %s/%s", c.o, to, rs, c.to, c.rs)
		}
	}
}
