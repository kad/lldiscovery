package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type Sender struct {
	multicastAddr string
	port          int
	interval      time.Duration
	logger        *slog.Logger
	tracer        trace.Tracer
	packetsSent   metric.Int64Counter
	errors        metric.Int64Counter
}

func NewSender(multicastAddr string, port int, interval time.Duration, logger *slog.Logger, packetsSent, errors metric.Int64Counter) *Sender {
	return &Sender{
		multicastAddr: multicastAddr,
		port:          port,
		interval:      interval,
		logger:        logger,
		tracer:        otel.Tracer("lldiscovery/discovery"),
		packetsSent:   packetsSent,
		errors:        errors,
	}
}

func (s *Sender) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.sendDiscovery()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.sendDiscovery()
		}
	}
}

func (s *Sender) sendDiscovery() {
	ctx, span := s.tracer.Start(context.Background(), "send_discovery")
	defer span.End()

	interfaces, err := GetActiveInterfaces()
	if err != nil {
		s.logger.Error("failed to get interfaces", "error", err)
		if s.errors != nil {
			s.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("error", "get_interfaces")))
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get interfaces")
		return
	}

	span.SetAttributes(attribute.Int("interface_count", len(interfaces)))

	for _, iface := range interfaces {
		if err := s.sendOnInterface(ctx, iface); err != nil {
			s.logger.Error("failed to send on interface",
				"interface", iface.Name,
				"error", err)
			if s.errors != nil {
				s.errors.Add(ctx, 1, metric.WithAttributes(
					attribute.String("error", "send_packet"),
					attribute.String("interface", iface.Name),
				))
			}
		}
	}
}

func (s *Sender) sendOnInterface(ctx context.Context, iface InterfaceInfo) error {
	ctx, span := s.tracer.Start(ctx, "send_on_interface",
		trace.WithAttributes(
			attribute.String("interface", iface.Name),
			attribute.String("source_ip", iface.LinkLocal),
		))
	defer span.End()

	packet, err := NewPacket(iface.Name, iface.LinkLocal)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create packet")
		return fmt.Errorf("create packet: %w", err)
	}
	
	// Add RDMA device info and GUIDs if available
	if iface.IsRDMA {
		packet.RDMADevice = iface.RDMADevice
		packet.NodeGUID = iface.NodeGUID
		packet.SysImageGUID = iface.SysImageGUID
	}

	data, err := packet.Marshal()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal packet")
		return fmt.Errorf("marshal packet: %w", err)
	}

	span.SetAttributes(attribute.Int("packet_size", len(data)))

	laddr, err := net.ResolveUDPAddr("udp6", "["+iface.LinkLocal+"]:0")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to resolve local address")
		return fmt.Errorf("resolve local addr: %w", err)
	}

	raddr, err := net.ResolveUDPAddr("udp6", fmt.Sprintf("[%s%%%s]:%d", s.multicastAddr, iface.Name, s.port))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to resolve multicast address")
		return fmt.Errorf("resolve multicast addr: %w", err)
	}

	conn, err := net.DialUDP("udp6", laddr, raddr)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to dial UDP")
		return fmt.Errorf("dial udp: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(data)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to write packet")
		return fmt.Errorf("write packet: %w", err)
	}

	if s.packetsSent != nil {
		s.packetsSent.Add(ctx, 1, metric.WithAttributes(
			attribute.String("interface", iface.Name),
		))
	}

	s.logger.Debug("sent discovery packet",
		"interface", iface.Name,
		"source", iface.LinkLocal,
		"size", len(data),
		"content", string(data))

	span.SetStatus(codes.Ok, "packet sent successfully")
	return nil
}
