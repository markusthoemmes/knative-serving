package main

import (
	"flag"
	"os"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/wait"

	fakeprov "github.com/knative/serving/pkg/metrics"
	basecmd "github.com/kubernetes-incubator/custom-metrics-apiserver/pkg/cmd"
	"github.com/kubernetes-incubator/custom-metrics-apiserver/pkg/provider"
)

type SampleAdapter struct {
	basecmd.AdapterBase
}

func (a *SampleAdapter) makeProviderOrDie() provider.CustomMetricsProvider {
	client, err := a.DynamicClient()
	if err != nil {
		glog.Fatalf("unable to construct dynamic client: %v", err)
	}

	mapper, err := a.RESTMapper()
	if err != nil {
		glog.Fatalf("unable to construct discovery REST mapper: %v", err)
	}

	return fakeprov.NewFakeProvider(client, mapper)
}

func main() {

	cmd := &SampleAdapter{}
	cmd.Flags().AddGoFlagSet(flag.CommandLine) // make sure we get the glog flags
	cmd.Flags().Parse(os.Args)

	testProvider := cmd.makeProviderOrDie()
	cmd.WithCustomMetrics(testProvider)

	if err := cmd.Run(wait.NeverStop); err != nil {
		glog.Fatalf("unable to run custom metrics adapter: %v", err)
	}
}
