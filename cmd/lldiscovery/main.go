package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/metric"
	"kad.name/lldiscovery/internal/config"
	"kad.name/lldiscovery/internal/discovery"
	"kad.name/lldiscovery/internal/export"
	"kad.name/lldiscovery/internal/graph"
	"kad.name/lldiscovery/internal/server"
	"kad.name/lldiscovery/internal/telemetry"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	
	configPath  = flag.String("config", "", "path to configuration file")
	logLevel    = flag.String("log-level", "", "log level (debug, info, warn, error)")
	showVersion = flag.Bool("version", false, "show version and exit")
	listRDMA    = flag.Bool("list-rdma", false, "list RDMA devices and their parent interfaces, then exit")
	
	// Timing parameters
	sendInterval   = flag.Duration("send-interval", 0, "how often to send discovery packets (e.g., 30s)")
	nodeTimeout    = flag.Duration("node-timeout", 0, "remove nodes after this period of no packets (e.g., 120s)")
	exportInterval = flag.Duration("export-interval", 0, "how often to check for changes and export (e.g., 60s)")
	
	// Network parameters
	multicastAddr = flag.String("multicast-address", "", "IPv6 multicast address (default: ff02::4c4c:6469)")
	multicastPort = flag.Int("multicast-port", 0, "UDP port for discovery protocol")
	
	// Output parameters
	outputFile  = flag.String("output-file", "", "path to DOT file output")
	httpAddress = flag.String("http-address", "", "HTTP server bind address (e.g., :8080)")
	
	// Feature flags
	includeNeighbors = flag.Bool("include-neighbors", false, "share neighbor information for transitive discovery")
	
	// Telemetry parameters
	telemetryEnabled       = flag.Bool("telemetry-enabled", false, "enable OpenTelemetry")
	telemetryEndpoint      = flag.String("telemetry-endpoint", "", "OpenTelemetry endpoint URL (e.g., grpc://localhost:4317, http://localhost:4318)")
	telemetryInsecure      = flag.Bool("telemetry-insecure", false, "use insecure connection for telemetry")
	telemetryEnableTraces  = flag.Bool("telemetry-traces", false, "enable trace export")
	telemetryEnableMetrics = flag.Bool("telemetry-metrics", false, "enable metrics export")
	telemetryEnableLogs    = flag.Bool("telemetry-logs", false, "enable logs export")
)

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("lldiscovery %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
		os.Exit(0)
	}

	if *listRDMA {
		listRDMADevices()
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Override config with CLI flags if provided
	if *logLevel != "" {
		cfg.LogLevel = *logLevel
	}
	if *sendInterval > 0 {
		cfg.SendInterval = *sendInterval
	}
	if *nodeTimeout > 0 {
		cfg.NodeTimeout = *nodeTimeout
	}
	if *exportInterval > 0 {
		cfg.ExportInterval = *exportInterval
	}
	if *multicastAddr != "" {
		cfg.MulticastAddr = *multicastAddr
	}
	if *multicastPort > 0 {
		cfg.MulticastPort = *multicastPort
	}
	if *outputFile != "" {
		cfg.OutputFile = *outputFile
	}
	if *httpAddress != "" {
		cfg.HTTPAddress = *httpAddress
	}
	// Note: includeNeighbors flag is false by default, so we need to check if it was explicitly set
	// We'll use a separate approach for boolean flags
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "include-neighbors" {
			cfg.IncludeNeighbors = *includeNeighbors
		}
		if f.Name == "telemetry-enabled" {
			cfg.Telemetry.Enabled = *telemetryEnabled
		}
		if f.Name == "telemetry-insecure" {
			cfg.Telemetry.Insecure = *telemetryInsecure
		}
		if f.Name == "telemetry-traces" {
			cfg.Telemetry.EnableTraces = *telemetryEnableTraces
		}
		if f.Name == "telemetry-metrics" {
			cfg.Telemetry.EnableMetrics = *telemetryEnableMetrics
		}
		if f.Name == "telemetry-logs" {
			cfg.Telemetry.EnableLogs = *telemetryEnableLogs
		}
	})
	if *telemetryEndpoint != "" {
		cfg.Telemetry.Endpoint = *telemetryEndpoint
	}
	
	// Parse endpoint URL to extract protocol and default ports
	if err := cfg.Telemetry.ParseEndpoint(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid telemetry endpoint: %v\n", err)
		os.Exit(1)
	}

	logger := setupLogger(cfg.LogLevel)
	logger.Info("starting lldiscovery",
		"version", version,
		"send_interval", cfg.SendInterval,
		"node_timeout", cfg.NodeTimeout,
		"export_interval", cfg.ExportInterval,
		"output_file", cfg.OutputFile,
		"telemetry_enabled", cfg.Telemetry.Enabled)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup telemetry
	telProvider, err := telemetry.Setup(ctx, telemetry.Config{
		Enabled:       cfg.Telemetry.Enabled,
		ServiceName:   "lldiscovery",
		Endpoint:      cfg.Telemetry.Endpoint,
		Protocol:      cfg.Telemetry.Protocol,
		Insecure:      cfg.Telemetry.Insecure,
		EnableTraces:  cfg.Telemetry.EnableTraces,
		EnableMetrics: cfg.Telemetry.EnableMetrics,
		EnableLogs:    cfg.Telemetry.EnableLogs,
	})
	if err != nil {
		logger.Error("failed to setup telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := telProvider.Shutdown(shutdownCtx); err != nil {
			logger.Error("failed to shutdown telemetry", "error", err)
		}
	}()

	// Initialize metrics
	var metrics *telemetry.Metrics
	if cfg.Telemetry.Enabled && cfg.Telemetry.EnableMetrics {
		metrics, err = telemetry.NewMetrics(ctx)
		if err != nil {
			logger.Error("failed to create metrics", "error", err)
			os.Exit(1)
		}
		logger.Info("metrics initialized")
	}

	g := graph.New()

	// Get local machine info and interfaces for the graph
	localInterfaces, err := discovery.GetActiveInterfaces()
	if err != nil {
		logger.Error("failed to get local interfaces", "error", err)
	} else {
		ifaceMap := make(map[string]graph.InterfaceDetails)
		for _, iface := range localInterfaces {
			ifaceMap[iface.Name] = graph.InterfaceDetails{
				IPAddress:    iface.LinkLocal,
				RDMADevice:   iface.RDMADevice,
				NodeGUID:     iface.NodeGUID,
				SysImageGUID: iface.SysImageGUID,
			}
		}
		
		// Get hostname and machine ID
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "unknown"
		}
		
		machineID, err := os.ReadFile("/etc/machine-id")
		if err == nil {
			g.SetLocalNode(strings.TrimSpace(string(machineID)), hostname, ifaceMap)
			logger.Info("local node added to graph",
				"hostname", hostname,
				"interfaces", len(ifaceMap))
		}
	}

	var packetsReceived, packetsSent, errors, multicastFailures metric.Int64Counter
	if metrics != nil {
		packetsReceived = metrics.PacketsReceived
		packetsSent = metrics.PacketsSent
		errors = metrics.DiscoveryErrors
		multicastFailures = metrics.MulticastJoinFailures
	}

	receiver, err := discovery.NewReceiver(cfg.MulticastAddr, cfg.MulticastPort, logger, func(p *discovery.Packet, sourceIP, receivingIface string) {
		// Add direct edge for received packet
		g.AddOrUpdate(p.MachineID, p.Hostname, p.Interface, sourceIP, receivingIface, p.RDMADevice, p.NodeGUID, p.SysImageGUID, true, "")
		
		// Process neighbors if included
		if cfg.IncludeNeighbors && len(p.Neighbors) > 0 {
			localMachineID := g.GetLocalMachineID()
			for _, neighbor := range p.Neighbors {
				// Skip if neighbor is local node (avoid self-loop)
				if neighbor.MachineID == localMachineID {
					continue
				}
				
				// Create indirect edge with full information from both sides
				// The neighbor struct contains: sender's interface to neighbor (Local*) and neighbor's interface (Remote*)
				// We create an edge from us to the neighbor, using the sender's local interface as the "remote" interface
				// because from our perspective, the sender's interface is the remote side
				g.AddOrUpdateIndirectEdge(
					neighbor.MachineID,
					neighbor.Hostname,
					neighbor.RemoteInterface,    // Neighbor's interface
					neighbor.RemoteAddress,      // Neighbor's address
					neighbor.RemoteRDMADevice,   // Neighbor's RDMA
					neighbor.RemoteNodeGUID,
					neighbor.RemoteSysImageGUID,
					neighbor.LocalInterface,     // Sender's interface (connecting to neighbor)
					neighbor.LocalAddress,       // Sender's address
					neighbor.LocalRDMADevice,    // Sender's RDMA
					neighbor.LocalNodeGUID,
					neighbor.LocalSysImageGUID,
					p.MachineID,                 // Learned from sender
				)
			}
		}
	}, packetsReceived, multicastFailures)
	if err != nil {
		logger.Error("failed to create receiver", "error", err)
		os.Exit(1)
	}

	sender := discovery.NewSender(cfg.MulticastAddr, cfg.MulticastPort, cfg.SendInterval, logger, packetsSent, errors, cfg.IncludeNeighbors, g)
	srv := server.New(cfg.HTTPAddress, g, logger)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 3)

	go func() {
		if err := receiver.Run(ctx); err != nil && err != context.Canceled {
			errChan <- fmt.Errorf("receiver: %w", err)
		}
	}()

	go func() {
		if err := sender.Run(ctx); err != nil && err != context.Canceled {
			errChan <- fmt.Errorf("sender: %w", err)
		}
	}()

	go func() {
		if err := srv.Run(ctx); err != nil && err != context.Canceled {
			errChan <- fmt.Errorf("server: %w", err)
		}
	}()

	go runExporter(ctx, g, cfg, logger, metrics)

	select {
	case sig := <-sigChan:
		logger.Info("received signal", "signal", sig)
		cancel()
	case err := <-errChan:
		logger.Error("component error", "error", err)
		cancel()
	}

	time.Sleep(100 * time.Millisecond)
	logger.Info("shutdown complete")
}

func runExporter(ctx context.Context, g *graph.Graph, cfg *config.Config, logger *slog.Logger, metrics *telemetry.Metrics) {
	exportTicker := time.NewTicker(cfg.ExportInterval)
	defer exportTicker.Stop()

	expireTicker := time.NewTicker(30 * time.Second)
	defer expireTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-exportTicker.C:
			if g.HasChanges() {
				nodes := g.GetNodes()
				edges := g.GetEdges()
				dot := export.GenerateDOT(nodes, edges)
				if err := export.WriteDOTFile(cfg.OutputFile, dot); err != nil {
					logger.Error("failed to write DOT file", "error", err)
				} else {
					logger.Info("exported graph", "nodes", len(nodes), "file", cfg.OutputFile)
					g.ClearChanges()
					if metrics != nil {
						metrics.GraphExports.Add(ctx, 1)
						metrics.NodesDiscovered.Add(ctx, int64(len(nodes)))
					}
				}
			}
		case <-expireTicker.C:
			removed := g.RemoveExpired(cfg.NodeTimeout)
			if removed > 0 {
				logger.Info("removed expired nodes", "count", removed)
				if metrics != nil {
					metrics.NodesExpired.Add(ctx, int64(removed))
				}
			}
		}
	}
}

func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})

	return slog.New(handler)
}

func listRDMADevices() {
	devices, err := discovery.GetRDMADevices()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(devices) == 0 {
		fmt.Println("No RDMA devices found")
		fmt.Println("\nNote: RDMA devices can be:")
		fmt.Println("  - Hardware: InfiniBand, RoCE adapters")
		fmt.Println("  - Software: RXE (RDMA over Converged Ethernet)")
		fmt.Println("\nTo create a software RXE device:")
		fmt.Println("  sudo rdma link add rxe0 type rxe netdev eth0")
		return
	}

	fmt.Printf("Found %d RDMA device(s):\n\n", len(devices))
	
	for rdmaDevice, parentIfaces := range devices {
		fmt.Printf("📡 %s\n", rdmaDevice)
		
		// Read additional info from sysfs
		nodeGUID := readSysfs(fmt.Sprintf("/sys/class/infiniband/%s/node_guid", rdmaDevice))
		sysImageGUID := readSysfs(fmt.Sprintf("/sys/class/infiniband/%s/sys_image_guid", rdmaDevice))
		nodeType := readSysfs(fmt.Sprintf("/sys/class/infiniband/%s/node_type", rdmaDevice))
		
		if nodeGUID != "" {
			fmt.Printf("   Node GUID:      %s\n", nodeGUID)
		}
		if sysImageGUID != "" {
			fmt.Printf("   Sys Image GUID: %s\n", sysImageGUID)
		}
		if nodeType != "" {
			// node_type file format is like "1: CA" or just "1"
			parts := strings.Split(nodeType, ":")
			typeNum := strings.TrimSpace(parts[0])
			
			typeDesc := ""
			if len(parts) > 1 {
				// Already has description
				typeDesc = " (" + strings.TrimSpace(parts[1]) + ")"
			} else {
				// Add description based on number
				switch typeNum {
				case "1":
					typeDesc = " (CA - Channel Adapter)"
				case "2":
					typeDesc = " (Switch)"
				case "3":
					typeDesc = " (Router)"
				}
			}
			fmt.Printf("   Node Type:      %s%s\n", typeNum, typeDesc)
		}
		
		fmt.Printf("   Parent interfaces:\n")
		for _, iface := range parentIfaces {
			fmt.Printf("      - %s\n", iface)
			
			// Try to get the interface's IP addresses
			if ifaceObj, err := discovery.GetActiveInterfaces(); err == nil {
				for _, info := range ifaceObj {
					if info.Name == iface {
						fmt.Printf("        IPv6 link-local: %s\n", info.LinkLocal)
						break
					}
				}
			}
		}
		fmt.Println()
	}
	
	fmt.Printf("Total: %d RDMA device(s) on %d network interface(s)\n", 
		len(devices), countUniqueInterfaces(devices))
}

func readSysfs(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func countUniqueInterfaces(devices map[string][]string) int {
	seen := make(map[string]bool)
	for _, ifaces := range devices {
		for _, iface := range ifaces {
			seen[iface] = true
		}
	}
	return len(seen)
}
