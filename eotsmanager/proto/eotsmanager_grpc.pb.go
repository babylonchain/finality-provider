// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package proto

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

// EOTSManagerClient is the client API for EOTSManager service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type EOTSManagerClient interface {
	Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error)
	// CreateKey generates and saves an EOTS key
	CreateKey(ctx context.Context, in *CreateKeyRequest, opts ...grpc.CallOption) (*CreateKeyResponse, error)
	// CreateMasterRandPair creates a pair of master secret/public randomness
	CreateMasterRandPair(ctx context.Context, in *CreateMasterRandPairRequest, opts ...grpc.CallOption) (*CreateMasterRandPairResponse, error)
	// KeyRecord returns the key record
	KeyRecord(ctx context.Context, in *KeyRecordRequest, opts ...grpc.CallOption) (*KeyRecordResponse, error)
	// SignEOTS signs an EOTS with the EOTS private key and the relevant randomness
	SignEOTS(ctx context.Context, in *SignEOTSRequest, opts ...grpc.CallOption) (*SignEOTSResponse, error)
	// SignSchnorrSig signs a Schnorr sig with the EOTS private key
	SignSchnorrSig(ctx context.Context, in *SignSchnorrSigRequest, opts ...grpc.CallOption) (*SignSchnorrSigResponse, error)
}

type eOTSManagerClient struct {
	cc grpc.ClientConnInterface
}

func NewEOTSManagerClient(cc grpc.ClientConnInterface) EOTSManagerClient {
	return &eOTSManagerClient{cc}
}

func (c *eOTSManagerClient) Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error) {
	out := new(PingResponse)
	err := c.cc.Invoke(ctx, "/proto.EOTSManager/Ping", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *eOTSManagerClient) CreateKey(ctx context.Context, in *CreateKeyRequest, opts ...grpc.CallOption) (*CreateKeyResponse, error) {
	out := new(CreateKeyResponse)
	err := c.cc.Invoke(ctx, "/proto.EOTSManager/CreateKey", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *eOTSManagerClient) CreateMasterRandPair(ctx context.Context, in *CreateMasterRandPairRequest, opts ...grpc.CallOption) (*CreateMasterRandPairResponse, error) {
	out := new(CreateMasterRandPairResponse)
	err := c.cc.Invoke(ctx, "/proto.EOTSManager/CreateMasterRandPair", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *eOTSManagerClient) KeyRecord(ctx context.Context, in *KeyRecordRequest, opts ...grpc.CallOption) (*KeyRecordResponse, error) {
	out := new(KeyRecordResponse)
	err := c.cc.Invoke(ctx, "/proto.EOTSManager/KeyRecord", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *eOTSManagerClient) SignEOTS(ctx context.Context, in *SignEOTSRequest, opts ...grpc.CallOption) (*SignEOTSResponse, error) {
	out := new(SignEOTSResponse)
	err := c.cc.Invoke(ctx, "/proto.EOTSManager/SignEOTS", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *eOTSManagerClient) SignSchnorrSig(ctx context.Context, in *SignSchnorrSigRequest, opts ...grpc.CallOption) (*SignSchnorrSigResponse, error) {
	out := new(SignSchnorrSigResponse)
	err := c.cc.Invoke(ctx, "/proto.EOTSManager/SignSchnorrSig", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// EOTSManagerServer is the server API for EOTSManager service.
// All implementations must embed UnimplementedEOTSManagerServer
// for forward compatibility
type EOTSManagerServer interface {
	Ping(context.Context, *PingRequest) (*PingResponse, error)
	// CreateKey generates and saves an EOTS key
	CreateKey(context.Context, *CreateKeyRequest) (*CreateKeyResponse, error)
	// CreateMasterRandPair creates a pair of master secret/public randomness
	CreateMasterRandPair(context.Context, *CreateMasterRandPairRequest) (*CreateMasterRandPairResponse, error)
	// KeyRecord returns the key record
	KeyRecord(context.Context, *KeyRecordRequest) (*KeyRecordResponse, error)
	// SignEOTS signs an EOTS with the EOTS private key and the relevant randomness
	SignEOTS(context.Context, *SignEOTSRequest) (*SignEOTSResponse, error)
	// SignSchnorrSig signs a Schnorr sig with the EOTS private key
	SignSchnorrSig(context.Context, *SignSchnorrSigRequest) (*SignSchnorrSigResponse, error)
	mustEmbedUnimplementedEOTSManagerServer()
}

// UnimplementedEOTSManagerServer must be embedded to have forward compatible implementations.
type UnimplementedEOTSManagerServer struct {
}

func (UnimplementedEOTSManagerServer) Ping(context.Context, *PingRequest) (*PingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Ping not implemented")
}
func (UnimplementedEOTSManagerServer) CreateKey(context.Context, *CreateKeyRequest) (*CreateKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateKey not implemented")
}
func (UnimplementedEOTSManagerServer) CreateMasterRandPair(context.Context, *CreateMasterRandPairRequest) (*CreateMasterRandPairResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateMasterRandPair not implemented")
}
func (UnimplementedEOTSManagerServer) KeyRecord(context.Context, *KeyRecordRequest) (*KeyRecordResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method KeyRecord not implemented")
}
func (UnimplementedEOTSManagerServer) SignEOTS(context.Context, *SignEOTSRequest) (*SignEOTSResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SignEOTS not implemented")
}
func (UnimplementedEOTSManagerServer) SignSchnorrSig(context.Context, *SignSchnorrSigRequest) (*SignSchnorrSigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SignSchnorrSig not implemented")
}
func (UnimplementedEOTSManagerServer) mustEmbedUnimplementedEOTSManagerServer() {}

// UnsafeEOTSManagerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to EOTSManagerServer will
// result in compilation errors.
type UnsafeEOTSManagerServer interface {
	mustEmbedUnimplementedEOTSManagerServer()
}

func RegisterEOTSManagerServer(s grpc.ServiceRegistrar, srv EOTSManagerServer) {
	s.RegisterService(&EOTSManager_ServiceDesc, srv)
}

func _EOTSManager_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EOTSManagerServer).Ping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.EOTSManager/Ping",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EOTSManagerServer).Ping(ctx, req.(*PingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EOTSManager_CreateKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateKeyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EOTSManagerServer).CreateKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.EOTSManager/CreateKey",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EOTSManagerServer).CreateKey(ctx, req.(*CreateKeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EOTSManager_CreateMasterRandPair_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateMasterRandPairRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EOTSManagerServer).CreateMasterRandPair(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.EOTSManager/CreateMasterRandPair",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EOTSManagerServer).CreateMasterRandPair(ctx, req.(*CreateMasterRandPairRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EOTSManager_KeyRecord_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(KeyRecordRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EOTSManagerServer).KeyRecord(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.EOTSManager/KeyRecord",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EOTSManagerServer).KeyRecord(ctx, req.(*KeyRecordRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EOTSManager_SignEOTS_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SignEOTSRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EOTSManagerServer).SignEOTS(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.EOTSManager/SignEOTS",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EOTSManagerServer).SignEOTS(ctx, req.(*SignEOTSRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _EOTSManager_SignSchnorrSig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SignSchnorrSigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EOTSManagerServer).SignSchnorrSig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.EOTSManager/SignSchnorrSig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EOTSManagerServer).SignSchnorrSig(ctx, req.(*SignSchnorrSigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// EOTSManager_ServiceDesc is the grpc.ServiceDesc for EOTSManager service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var EOTSManager_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "proto.EOTSManager",
	HandlerType: (*EOTSManagerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler:    _EOTSManager_Ping_Handler,
		},
		{
			MethodName: "CreateKey",
			Handler:    _EOTSManager_CreateKey_Handler,
		},
		{
			MethodName: "CreateMasterRandPair",
			Handler:    _EOTSManager_CreateMasterRandPair_Handler,
		},
		{
			MethodName: "KeyRecord",
			Handler:    _EOTSManager_KeyRecord_Handler,
		},
		{
			MethodName: "SignEOTS",
			Handler:    _EOTSManager_SignEOTS_Handler,
		},
		{
			MethodName: "SignSchnorrSig",
			Handler:    _EOTSManager_SignSchnorrSig_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "eotsmanager.proto",
}
