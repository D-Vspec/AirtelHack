// Command scoring is a Phase 0 skeleton. See prompts/ for the phase that fills it in.
package main

import "udyam/pkg/svc"

func main() {
	svc.Run(svc.Options{Name: "scoring", DefaultPort: "8087", Schema: "scoring", UsesKafka: true})
}
