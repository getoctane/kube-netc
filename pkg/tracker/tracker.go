package tracker

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/DataDog/datadog-agent/pkg/ebpf"
	"github.com/DataDog/datadog-agent/pkg/network"
	"go.uber.org/zap"
)

const (
	MaxConnBuffer = 256
)

func (t *Tracker) check(err error) {
	if err != nil {
		t.Logger.Fatalw(err.Error(),
			"package", "tracker",
		)
	}
}

type Tracker struct {
	// time idle before considering connection inactive
	Timeout        time.Duration
	Config         *ebpf.Config
	numConnections uint16

	Logger *zap.SugaredLogger

	tracer *ebpf.Tracer
}

type ConnData struct {
	BytesSent          uint64
	BytesRecv          uint64
	BytesSentPerSecond uint64
	BytesRecvPerSecond uint64
	Active             bool
	LastUpdated        time.Time
}

// Simple struct to pipe for any extra metrics not related to connections
// but rather the node as a whole
type NodeUpdate struct {
	NumConnections uint16
}

// Type to be piped through chan to collector for updates
type ConnUpdate struct {
	Connection ConnectionID
	Data       ConnData
}

type ConnectionID struct {
	DAddr string
	DPort uint16
	SAddr string
}

var (
	DefaultTracker = Tracker{
		Config: &ebpf.Config{
			CollectTCPConns:              true,
			CollectUDPConns:              true,
			CollectIPv6Conns:             true,
			CollectLocalDNS:              false,
			DNSInspection:                false,
			UDPConnTimeout:               30 * time.Second,
			TCPConnTimeout:               2 * time.Minute,
			MaxTrackedConnections:        65536,
			ConntrackMaxStateSize:        65536,
			ProcRoot:                     "/proc",
			BPFDebug:                     false,
			EnableConntrack:              true,
			MaxClosedConnectionsBuffered: 50000,
			MaxConnectionsStateBuffered:  75000,
			ClientStateExpiry:            2 * time.Minute,
			ClosedChannelSize:            500,
		},
		numConnections: 0,
	}
)

func NewTracker(logger *zap.SugaredLogger) *Tracker {
	dt := &DefaultTracker
	dt.Logger = logger
	return &DefaultTracker
}

func (t *Tracker) StartTracker() {
	err := checkSupport()
	t.check(err)
	t.Logger.Debugw("finished checking for eBPF suport",
		"package", "tracker",
	)

	tracer, err := ebpf.NewTracer(t.Config)
	if err != nil {
		panic(err)
	}
	t.tracer = tracer
}

func checkSupport() error {
	_, err := ebpf.CurrentKernelVersion()
	if err != nil {
		return err
	}

	if supported, errtip := ebpf.IsTracerSupportedByOS(nil); !supported {
		return errors.New(errtip)
	}
	return nil
}

func (t *Tracker) GetConns() (uint16, []ConnUpdate, error) {
	cs, err := t.tracer.GetActiveConnections(fmt.Sprintf("%d", os.Getpid()))
	if err != nil {
		return 0, nil, err
	}

	conns := cs.Conns

	connectionMap := make(map[ConnectionID][]network.ConnectionStats)

	for _, c := range conns {
		id := ConnectionID{
			SAddr: c.Source.String(),
			DAddr: c.Dest.String(),
			DPort: c.DPort,
		}

		if _, exists := connectionMap[id]; !exists {
			connectionMap[id] = []network.ConnectionStats{}
		}
		connectionMap[id] = append(connectionMap[id], c)
	}

	updates := make([]ConnUpdate, len(connectionMap))

	i := 0
	for id, conns := range connectionMap {
		update := ConnUpdate{
			Connection: id,
			Data: ConnData{
				Active:      true,
				LastUpdated: time.Now(),
			},
		}

		for _, c := range conns {
			// These values get used mored than once in calculations
			// and we want them to be uniform in this scope
			bytesSent := c.MonotonicSentBytes
			bytesRecv := c.MonotonicRecvBytes

			// Using runtime.nanotime(), see util.go
			now := Now()
			// In float64 seconds
			timeDiff := float64(now-c.LastUpdateEpoch) / 1000000000.0

			if timeDiff <= 0 {
				t.check(errors.New("no difference between LastUpdateEpoch and time.Now(), will create divide by zero error or negative"))
			}

			// Per Second Calculations
			bytesSentPerSecond := uint64(float64(bytesSent-c.LastSentBytes) / float64(timeDiff))
			bytesRecvPerSecond := uint64(float64(bytesRecv-c.LastRecvBytes) / float64(timeDiff))

			update.Data.BytesSent += c.MonotonicSentBytes
			update.Data.BytesRecv += c.MonotonicRecvBytes
			update.Data.BytesSentPerSecond += bytesSentPerSecond
			update.Data.BytesRecvPerSecond += bytesRecvPerSecond
		}

		updates[i] = update

		i++
	}

	return uint16(len(conns)), updates, nil
}
