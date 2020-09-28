package main

import (
	"net/http"
	"time"

	"github.com/getoctane/kube-netc/pkg/collector"
	"github.com/getoctane/kube-netc/pkg/tracker"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	t := tracker.NewTracker()
	go t.StartTracker()
	go collector.StartCollector(t, 2*time.Second)

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}
