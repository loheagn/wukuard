// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package grpc

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// SyncNetClient is the client API for SyncNet service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SyncNetClient interface {
	// HeartBeat : client sends info about itself to server
	// and server returns all information about the current network
	HeartBeat(ctx context.Context, in *Peer, opts ...grpc.CallOption) (*NetWork, error)
}

type syncNetClient struct {
	cc grpc.ClientConnInterface
}

func NewSyncNetClient(cc grpc.ClientConnInterface) SyncNetClient {
	return &syncNetClient{cc}
}

func (c *syncNetClient) HeartBeat(ctx context.Context, in *Peer, opts ...grpc.CallOption) (*NetWork, error) {
	out := new(NetWork)
	err := c.cc.Invoke(ctx, "/grpc.SyncNet/HeartBeat", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SyncNetServer is the server API for SyncNet service.
// All implementations must embed UnimplementedSyncNetServer
// for forward compatibility
type SyncNetServer interface {
	// HeartBeat : client sends info about itself to server
	// and server returns all information about the current network
	HeartBeat(context.Context, *Peer) (*NetWork, error)
	mustEmbedUnimplementedSyncNetServer()
}

// UnimplementedSyncNetServer must be embedded to have forward compatible implementations.
type UnimplementedSyncNetServer struct {
}

func (UnimplementedSyncNetServer) HeartBeat(context.Context, *Peer) (*NetWork, error) {
	return nil, status.Errorf(codes.Unimplemented, "method HeartBeat not implemented")
}
func (UnimplementedSyncNetServer) mustEmbedUnimplementedSyncNetServer() {}

// UnsafeSyncNetServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SyncNetServer will
// result in compilation errors.
type UnsafeSyncNetServer interface {
	mustEmbedUnimplementedSyncNetServer()
}

func RegisterSyncNetServer(s grpc.ServiceRegistrar, srv SyncNetServer) {
	s.RegisterService(&SyncNet_ServiceDesc, srv)
}

func _SyncNet_HeartBeat_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Peer)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncNetServer).HeartBeat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/grpc.SyncNet/HeartBeat",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncNetServer).HeartBeat(ctx, req.(*Peer))
	}
	return interceptor(ctx, in, info, handler)
}

// SyncNet_ServiceDesc is the grpc.ServiceDesc for SyncNet service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SyncNet_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "grpc.SyncNet",
	HandlerType: (*SyncNetServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "HeartBeat",
			Handler:    _SyncNet_HeartBeat_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "grpc/wukuard.proto",
}
