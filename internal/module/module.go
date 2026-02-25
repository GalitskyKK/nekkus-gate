package module

import (
	"context"
	"fmt"

	pb "github.com/GalitskyKK/nekkus-core/pkg/protocol"
	"google.golang.org/grpc"
)

// GateModule реализует NekkusModule для DNS-блокировки.
type GateModule struct {
	pb.UnimplementedNekkusModuleServer
	httpPort int
}

// New создаёт GateModule.
func New(httpPort int) *GateModule {
	if httpPort <= 0 {
		httpPort = 9003
	}
	return &GateModule{httpPort: httpPort}
}

func (m *GateModule) GetInfo(ctx context.Context, _ *pb.Empty) (*pb.ModuleInfo, error) {
	return &pb.ModuleInfo{
		Id:           "gate",
		Name:         "Nekkus Gate",
		Version:      "0.1.0",
		Description:  "DNS-level blocking: ads and trackers",
		Color:        "#EF4444",
		HttpPort:     int32(m.httpPort),
		GrpcPort:     19003,
		UiUrl:        fmt.Sprintf("http://127.0.0.1:%d", m.httpPort),
		Capabilities: []string{"dns.block", "dns.stats"},
		Provides:     []string{"dns.stats", "dns.blocklist"},
		Status:       pb.ModuleStatus_MODULE_RUNNING,
	}, nil
}

func (m *GateModule) Health(ctx context.Context, _ *pb.Empty) (*pb.HealthStatus, error) {
	return &pb.HealthStatus{
		Healthy: true,
		Message: "ok",
		Details: map[string]string{},
	}, nil
}

func (m *GateModule) GetWidgets(ctx context.Context, _ *pb.Empty) (*pb.WidgetList, error) {
	return &pb.WidgetList{
		Widgets: []*pb.Widget{
			{
				Id:                "gate.stats",
				Title:             "Gate",
				Size:              pb.WidgetSize_WIDGET_SMALL,
				DataEndpoint:      "/api/stats",
				RefreshIntervalMs: 3000,
			},
		},
	}, nil
}

func (m *GateModule) GetActions(ctx context.Context, _ *pb.Empty) (*pb.ActionList, error) {
	return &pb.ActionList{Actions: []*pb.Action{}}, nil
}

func (m *GateModule) StreamData(*pb.StreamRequest, grpc.ServerStreamingServer[pb.DataEvent]) error {
	return nil
}

func (m *GateModule) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	return &pb.QueryResponse{Success: false, Error: "unknown query"}, nil
}

func (m *GateModule) Execute(ctx context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
	return &pb.ExecuteResponse{Success: false, Error: "unknown action"}, nil
}

func (m *GateModule) GetSnapshot(ctx context.Context, _ *pb.Empty) (*pb.StateSnapshot, error) {
	return &pb.StateSnapshot{ModuleId: "gate", Timestamp: 0, State: nil}, nil
}

func (m *GateModule) RestoreSnapshot(ctx context.Context, snap *pb.StateSnapshot) (*pb.RestoreResult, error) {
	return &pb.RestoreResult{Success: true, Message: "ok"}, nil
}
