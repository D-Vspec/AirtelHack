// Command tracker is a Phase 0 skeleton. See prompts/ for the phase that fills it in.
package main

import "udyam/pkg/svc"

func main() {
	svc.Run(svc.Options{Name: "tracker", DefaultPort: "8085", Schema: "tracker", UsesKafka: true})
}
