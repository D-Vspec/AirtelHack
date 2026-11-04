// Command kyc is a Phase 0 skeleton. See prompts/ for the phase that fills it in.
package main

import "udyam/pkg/svc"

func main() {
	svc.Run(svc.Options{Name: "kyc", DefaultPort: "8084", Schema: "kyc", UsesKafka: true})
}
