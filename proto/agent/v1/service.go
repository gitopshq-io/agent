package agentv1

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	AgentHubRegisterFullMethodName = "/agent.v1.AgentHub/Register"
	AgentHubConnectFullMethodName  = "/agent.v1.AgentHub/Connect"
)

type AgentHubClient interface {
	Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error)
	Connect(ctx context.Context, opts ...grpc.CallOption) (AgentHub_ConnectClient, error)
}

type agentHubClient struct {
	cc grpc.ClientConnInterface
}

func NewAgentHubClient(cc grpc.ClientConnInterface) AgentHubClient {
	return &agentHubClient{cc: cc}
}

func (c *agentHubClient) Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error) {
	out := new(RegisterResponse)
	if err := c.cc.Invoke(ctx, AgentHubRegisterFullMethodName, in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentHubClient) Connect(ctx context.Context, opts ...grpc.CallOption) (AgentHub_ConnectClient, error) {
	stream, err := c.cc.NewStream(ctx, &AgentHub_ServiceDesc.Streams[0], AgentHubConnectFullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	return &agentHubConnectClient{ClientStream: stream}, nil
}

type AgentHub_ConnectClient interface {
	Send(*AgentEnvelope) error
	Recv() (*HubEnvelope, error)
	grpc.ClientStream
}

type agentHubConnectClient struct {
	grpc.ClientStream
}

func (x *agentHubConnectClient) Send(m *AgentEnvelope) error {
	return x.ClientStream.SendMsg(m)
}

func (x *agentHubConnectClient) Recv() (*HubEnvelope, error) {
	m := new(HubEnvelope)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type AgentHubServer interface {
	Register(context.Context, *RegisterRequest) (*RegisterResponse, error)
	Connect(AgentHub_ConnectServer) error
}

type UnimplementedAgentHubServer struct{}

func (UnimplementedAgentHubServer) Register(context.Context, *RegisterRequest) (*RegisterResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Register not implemented")
}

func (UnimplementedAgentHubServer) Connect(AgentHub_ConnectServer) error {
	return status.Error(codes.Unimplemented, "method Connect not implemented")
}

func RegisterAgentHubServer(s grpc.ServiceRegistrar, srv AgentHubServer) {
	s.RegisterService(&AgentHub_ServiceDesc, srv)
}

type AgentHub_ConnectServer interface {
	Send(*HubEnvelope) error
	Recv() (*AgentEnvelope, error)
	grpc.ServerStream
}

func _AgentHub_Register_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(RegisterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentHubServer).Register(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: AgentHubRegisterFullMethodName,
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(AgentHubServer).Register(ctx, req.(*RegisterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AgentHub_Connect_Handler(srv any, stream grpc.ServerStream) error {
	return srv.(AgentHubServer).Connect(&agentHubConnectServer{ServerStream: stream})
}

type agentHubConnectServer struct {
	grpc.ServerStream
}

func (x *agentHubConnectServer) Send(m *HubEnvelope) error {
	return x.ServerStream.SendMsg(m)
}

func (x *agentHubConnectServer) Recv() (*AgentEnvelope, error) {
	m := new(AgentEnvelope)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var AgentHub_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "agent.v1.AgentHub",
	HandlerType: (*AgentHubServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Register",
			Handler:    _AgentHub_Register_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Connect",
			Handler:       _AgentHub_Connect_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "proto/agent/v1/agent.proto",
}
