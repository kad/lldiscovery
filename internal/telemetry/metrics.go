package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	PacketsSent          metric.Int64Counter
	PacketsReceived      metric.Int64Counter
	NodesDiscovered      metric.Int64UpDownCounter
	InterfacesActive     metric.Int64UpDownCounter
	GraphExports         metric.Int64Counter
	NodesExpired         metric.Int64Counter
	DiscoveryErrors      metric.Int64Counter
	MulticastJoinFailures metric.Int64Counter
}

func NewMetrics(ctx context.Context) (*Metrics, error) {
	meter := otel.Meter("lldiscovery")

	packetsSent, err := meter.Int64Counter(
		"lldiscovery.packets.sent",
		metric.WithDescription("Number of discovery packets sent"),
		metric.WithUnit("{packet}"),
	)
	if err != nil {
		return nil, err
	}

	packetsReceived, err := meter.Int64Counter(
		"lldiscovery.packets.received",
		metric.WithDescription("Number of discovery packets received"),
		metric.WithUnit("{packet}"),
	)
	if err != nil {
		return nil, err
	}

	nodesDiscovered, err := meter.Int64UpDownCounter(
		"lldiscovery.nodes.discovered",
		metric.WithDescription("Current number of discovered nodes"),
		metric.WithUnit("{node}"),
	)
	if err != nil {
		return nil, err
	}

	interfacesActive, err := meter.Int64UpDownCounter(
		"lldiscovery.interfaces.active",
		metric.WithDescription("Number of active interfaces"),
		metric.WithUnit("{interface}"),
	)
	if err != nil {
		return nil, err
	}

	graphExports, err := meter.Int64Counter(
		"lldiscovery.graph.exports",
		metric.WithDescription("Number of graph exports"),
		metric.WithUnit("{export}"),
	)
	if err != nil {
		return nil, err
	}

	nodesExpired, err := meter.Int64Counter(
		"lldiscovery.nodes.expired",
		metric.WithDescription("Number of nodes that expired due to timeout"),
		metric.WithUnit("{node}"),
	)
	if err != nil {
		return nil, err
	}

	discoveryErrors, err := meter.Int64Counter(
		"lldiscovery.errors.discovery",
		metric.WithDescription("Number of discovery errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	multicastJoinFailures, err := meter.Int64Counter(
		"lldiscovery.errors.multicast_join",
		metric.WithDescription("Number of multicast join failures"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		PacketsSent:           packetsSent,
		PacketsReceived:       packetsReceived,
		NodesDiscovered:       nodesDiscovered,
		InterfacesActive:      interfacesActive,
		GraphExports:          graphExports,
		NodesExpired:          nodesExpired,
		DiscoveryErrors:       discoveryErrors,
		MulticastJoinFailures: multicastJoinFailures,
	}, nil
}
