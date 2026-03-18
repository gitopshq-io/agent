package hubgrpc

import (
	"context"

	"github.com/gitopshq-io/agent/internal/domain"
	cfgpkg "github.com/gitopshq-io/agent/internal/platform/config"
	"github.com/gitopshq-io/agent/internal/port"
	agentv1 "github.com/gitopshq-io/agent/proto/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	cfg cfgpkg.HubConfig
}

func New(cfg cfgpkg.HubConfig) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) Register(ctx context.Context, token string, cluster domain.Cluster) (domain.RegisterResponse, error) {
	conn, err := c.dial()
	if err != nil {
		return domain.RegisterResponse{}, err
	}
	defer conn.Close()
	client := agentv1.NewAgentHubClient(conn)
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	resp, err := client.Register(ctx, &agentv1.RegisterRequest{
		RegistrationToken: token,
		Cluster:           toProtoClusterInfo(cluster),
	})
	if err != nil {
		return domain.RegisterResponse{}, err
	}
	return fromProtoRegisterResponse(resp), nil
}

func (c *Client) Connect(ctx context.Context, agentToken string) (port.HubSession, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	client := agentv1.NewAgentHubClient(conn)
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+agentToken)
	stream, err := client.Connect(ctx)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &session{conn: conn, stream: stream}, nil
}

func (c *Client) dial() (*grpc.ClientConn, error) {
	var transportCreds credentials.TransportCredentials
	if c.cfg.Insecure {
		transportCreds = insecure.NewCredentials()
	} else {
		tlsCfg, err := c.cfg.TLSConfig()
		if err != nil {
			return nil, err
		}
		transportCreds = credentials.NewTLS(tlsCfg)
	}
	return grpc.NewClient(
		c.cfg.Address,
		grpc.WithTransportCredentials(transportCreds),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(agentv1.JSONCodecName)),
	)
}

type session struct {
	conn   *grpc.ClientConn
	stream agentv1.AgentHub_ConnectClient
}

var _ port.HubSession = (*session)(nil)

func (s *session) Send(msg domain.AgentMessage) error {
	return s.stream.Send(toProtoAgentEnvelope(msg))
}

func (s *session) Recv() (domain.HubMessage, error) {
	msg, err := s.stream.Recv()
	if err != nil {
		return domain.HubMessage{}, err
	}
	return fromProtoHubEnvelope(msg), nil
}

func (s *session) CloseSend() error {
	err := s.stream.CloseSend()
	_ = s.conn.Close()
	return err
}
