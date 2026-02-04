package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/net/ipv6"
)

type PacketHandler func(*Packet, string, string)

type Receiver struct {
	multicastAddr     string
	port              int
	logger            *slog.Logger
	handler           PacketHandler
	localMachineID    string
	tracer            trace.Tracer
	packetsReceived   metric.Int64Counter
	multicastFailures metric.Int64Counter
}

func NewReceiver(multicastAddr string, port int, logger *slog.Logger, handler PacketHandler, packetsReceived, multicastFailures metric.Int64Counter) (*Receiver, error) {
	machineID, err := readMachineID()
	if err != nil {
		return nil, fmt.Errorf("read machine-id: %w", err)
	}

	return &Receiver{
		multicastAddr:     multicastAddr,
		port:              port,
		logger:            logger,
		handler:           handler,
		localMachineID:    machineID,
		tracer:            otel.Tracer("lldiscovery/discovery"),
		packetsReceived:   packetsReceived,
		multicastFailures: multicastFailures,
	}, nil
}

func (r *Receiver) Run(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", r.port)
	laddr, err := net.ResolveUDPAddr("udp6", addr)
	if err != nil {
		return fmt.Errorf("resolve addr: %w", err)
	}

	conn, err := net.ListenUDP("udp6", laddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer conn.Close()

	interfaces, err := GetActiveInterfaces()
	if err != nil {
		return fmt.Errorf("get interfaces: %w", err)
	}

	p := ipv6.NewPacketConn(conn)
	
	// Enable control messages to get receiving interface info
	if err := p.SetControlMessage(ipv6.FlagInterface, true); err != nil {
		r.logger.Warn("failed to enable interface control messages", "error", err)
	}
	
	for _, iface := range interfaces {
		ifaceObj, err := net.InterfaceByName(iface.Name)
		if err != nil {
			r.logger.Warn("failed to get interface",
				"interface", iface.Name,
				"error", err)
			continue
		}

		group := net.ParseIP(r.multicastAddr)
		if err := p.JoinGroup(ifaceObj, &net.UDPAddr{IP: group}); err != nil {
			r.logger.Warn("failed to join multicast group",
				"interface", iface.Name,
				"group", r.multicastAddr,
				"error", err)
			if r.multicastFailures != nil {
				r.multicastFailures.Add(context.Background(), 1, metric.WithAttributes(
					attribute.String("interface", iface.Name),
				))
			}
		} else {
			r.logger.Info("joined multicast group",
				"interface", iface.Name,
				"group", r.multicastAddr)
		}
	}

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	buf := make([]byte, 65536)
	for {
		n, cm, src, err := p.ReadFrom(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				r.logger.Error("read error", "error", err)
				continue
			}
		}

		var receivingInterface string
		if cm != nil && cm.IfIndex > 0 {
			if iface, err := net.InterfaceByIndex(cm.IfIndex); err == nil {
				receivingInterface = iface.Name
			}
		}

		if udpAddr, ok := src.(*net.UDPAddr); ok {
			r.handlePacket(buf[:n], udpAddr, receivingInterface)
		}
	}
}

func (r *Receiver) handlePacket(data []byte, remoteAddr *net.UDPAddr, receivingInterface string) {
	ctx, span := r.tracer.Start(context.Background(), "handle_packet")
	defer span.End()

	packet, err := UnmarshalPacket(data)
	if err != nil {
		r.logger.Warn("failed to unmarshal packet", "error", err)
		span.RecordError(err)
		return
	}

	span.SetAttributes(
		attribute.String("hostname", packet.Hostname),
		attribute.String("machine_id", packet.MachineID[:8]),
		attribute.String("interface", packet.Interface),
	)

	if packet.MachineID == r.localMachineID {
		span.AddEvent("ignored_own_packet")
		return
	}

	sourceIP := remoteAddr.IP.String()
	if idx := strings.Index(sourceIP, "%"); idx != -1 {
		sourceIP = sourceIP[:idx]
	}

	if r.packetsReceived != nil {
		r.packetsReceived.Add(ctx, 1, metric.WithAttributes(
			attribute.String("hostname", packet.Hostname),
			attribute.String("interface", packet.Interface),
		))
	}

	r.logger.Debug("received discovery packet",
		"hostname", packet.Hostname,
		"machine_id", packet.MachineID[:8],
		"source", sourceIP,
		"sender_interface", packet.Interface,
		"received_on", receivingInterface,
		"content", string(data))

	if r.handler != nil {
		r.handler(packet, sourceIP, receivingInterface)
	}
}
