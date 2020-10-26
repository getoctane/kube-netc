package collector

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/getoctane/kube-netc/pkg/cluster"
	"github.com/getoctane/kube-netc/pkg/tracker"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func getEmpty() *cluster.ObjectInfo {
	return &cluster.ObjectInfo{
		Name:      "",
		Kind:      "",
		Namespace: "",
		Node:      "",
		Zone:      "",
		// LoadBalancerIP: "",

		LabelName:      "",
		LabelComponent: "",
		LabelInstance:  "",
		LabelVersion:   "",
		LabelPartOf:    "",
		LabelManagedBy: "",
	}
}

type Collector struct {
	tr     *tracker.Tracker
	ci     *cluster.ClusterInfo
	logger *zap.SugaredLogger
}

func NewCollector(tr *tracker.Tracker, ci *cluster.ClusterInfo, logger *zap.SugaredLogger) *Collector {
	return &Collector{tr, ci, logger}
}

func (c *Collector) updateNodeMetrics(numConns uint16) {
	c.logger.Debugw("updating num connections",
		"package", "collector",
		"num_conns", int(numConns),
	)
	ActiveConnections.Set(float64(numConns))
}

func (c *Collector) updateConnMetrics(connUpdates []tracker.ConnUpdate) {
	updatesBySrcIP := make(map[string][]tracker.ConnUpdate)

	for _, update := range connUpdates {
		if update.Data.BytesSentPerSecond > 50e9 {
			c.logger.Warnw("extreme transfer rate",
				"package", "tracker",
				"source", update.Connection.SAddr,
				"direction", "sent",
				"bps", update.Data.BytesSentPerSecond,
			)
		} else if update.Data.BytesRecvPerSecond > 50e9 {
			c.logger.Warnw("extreme transfer rate",
				"package", "tracker",
				"source", update.Connection.SAddr,
				"direction", "recv",
				"bps", update.Data.BytesRecvPerSecond,
			)
		}

		if _, exists := updatesBySrcIP[update.Connection.SAddr]; !exists {
			updatesBySrcIP[update.Connection.SAddr] = []tracker.ConnUpdate{}
		}
		updatesBySrcIP[update.Connection.SAddr] = append(updatesBySrcIP[update.Connection.SAddr], update)
	}

	for _, updates := range updatesBySrcIP {
		labels := c.generateLabels(updates[0])

		aggregateData := tracker.ConnData{}

		for _, update := range updates {
			aggregateData.BytesSent += update.Data.BytesSent
			aggregateData.BytesRecv += update.Data.BytesRecv
			aggregateData.BytesSentPerSecond += update.Data.BytesSentPerSecond
			aggregateData.BytesRecvPerSecond += update.Data.BytesRecvPerSecond
		}

		BytesSent.With(labels).Set(float64(aggregateData.BytesSent))
		BytesRecv.With(labels).Set(float64(aggregateData.BytesRecv))
		BytesSentPerSecond.With(labels).Set(float64(aggregateData.BytesSentPerSecond))
		BytesRecvPerSecond.With(labels).Set(float64(aggregateData.BytesRecvPerSecond))
	}
}

func trafficType(srcInfo *cluster.ObjectInfo, dstInfo *cluster.ObjectInfo, dAddr string) string {
	// Traffic TO kube-proxy from pod: mark as internet (so it is not marked as
	// intra_zone)
	if srcInfo.Kind == "pod" && strings.Contains(dstInfo.Name, "kube-proxy") {
		return "internet"
	}

	// Inter-zone if zones do not much (but they are provided)
	if srcInfo.Zone != "" && dstInfo.Zone != "" && srcInfo.Zone != dstInfo.Zone {
		return "inter_zone"
	}

	// Else we should take a look at the IP
	dstIP, _, err := net.ParseCIDR(dAddr + "/32")
	if err != nil {
		fmt.Println(fmt.Errorf("CIDR parse error on %q: %v", dAddr, err))
		return "" // Just return unknown
	}

	if isLoopbackIP(dstIP) {
		return "loopback"
	}

	if isPrivateIP(dstIP) {
		return "intra_zone"
	}

	// NOTE ----------------------------------------------------------------------
	// Everything below occurs after the IP has been identified as a public address.

	// Traffic FROM kube-proxy out of cluster: mark as intra_zone (so it is not
	// marked as internet)
	if dstInfo.Kind == "" && strings.Contains(srcInfo.Name, "kube-proxy") {
		return "intra_zone"
	}

	return "internet"
}

func (c *Collector) generateLabels(update tracker.ConnUpdate) prometheus.Labels {
	conn := update.Connection

	srcInfo, sok := c.ci.Get(conn.SAddr)
	if !sok {
		srcInfo = getEmpty()
	}

	destInfo, dok := c.ci.Get(conn.DAddr)
	if !dok {
		destInfo = getEmpty()
	}

	return prometheus.Labels{
		"source_address":   conn.SAddr,
		"source_name":      srcInfo.Name,
		"source_kind":      srcInfo.Kind,
		"source_namespace": srcInfo.Namespace,
		"source_node":      srcInfo.Node,
		"traffic_type":     trafficType(srcInfo, destInfo, conn.DAddr),
	}
}

func (c *Collector) Start() {
	ticker := time.NewTicker(1 * time.Minute).C

	c.logger.Debugw("starting tracker control loop",
		"package", "tracker",
	)

	for {
		select {

		case <-ticker:
			numConns, connUpdates, err := c.tr.GetConns()
			if err != nil {
				c.logger.Fatalw(err.Error(),
					"package", "collector",
				)
				continue
			}
			c.updateNodeMetrics(numConns)
			c.updateConnMetrics(connUpdates)
		}
	}
}
