package main

import (
	"k8s.io/apimachinery/pkg/util/wait"

	basecmd "github.com/kubernetes-incubator/custom-metrics-apiserver/pkg/cmd"
)

type YourAdapter struct {
	basecmd.AdapterBase
}

func main() {
	// initialize the flags, with one custom flag for the message
	cmd := &YourAdapter{}

	// provider := cmd.makeProviderOrDie()
	//cmd.WithCustomMetrics(provider)
	// you could also set up external metrics support,
	// if your provider supported it:
	// cmd.WithExternalMetrics(provider)

	if err := cmd.Run(wait.NeverStop); err != nil {
		panic("unable to run custom metrics adapter: %v", err)
	}
}
