package grpcmock

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Server struct {
	matchers []*matcher
	// fds               linker.Files
	listener net.Listener
	server   *grpc.Server
	// tlsc              *tls.Config
	// cacert            []byte
	cc *grpc.ClientConn
	// requests          []*Request
	requests []*dynamicpb.Message
	// unmatchedRequests []*Request
	unmatchedRequests []protoreflect.ProtoMessage
	// healthCheck       bool
	// disableReflection bool
	// status            serverStatus
	t  TB
	mu sync.RWMutex
}

var _ TB = (testing.TB)(nil)

type TB interface {
	Error(args ...any)
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Helper()
}

type matcher struct {
	serviceName  string
	methodName   string
	requestType  protoreflect.ProtoMessage
	responseType protoreflect.ProtoMessage
	// matchFuncs   []matchFunc
	handler handlerFunc
	// requests   []*Request
	requests []*dynamicpb.Message
	t        TB
	mu       sync.RWMutex
}

// type matchFunc func(r *Request) bool
type matchFunc func(r protoreflect.ProtoMessage) bool

// type handlerFunc func(r *Request, md protoreflect.MethodDescriptor) *Response
type handlerFunc func(r protoreflect.ProtoMessage) protoreflect.ProtoMessage

func NewServer(t TB) *Server {
	t.Helper()
	s := &Server{}
	s.server = grpc.NewServer()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
		return nil
	}
	s.listener = lis
	s.t = t
	return s
}

func (s *Server) Start() {
	go s.server.Serve(s.listener)
}

func (s *Server) ClientConn() *grpc.ClientConn {
	if s.cc != nil {
		return s.cc
	}
	cc, err := grpc.Dial(s.listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		s.t.Fatal(err)
		return nil
	}
	s.cc = cc
	return s.cc
}

func (s *Server) Register(serviceName, methodName string, reqType protoreflect.ProtoMessage, respType protoreflect.ProtoMessage) *matcher {
	m := &matcher{
		serviceName:  serviceName,
		methodName:   methodName,
		requestType:  reqType,
		responseType: respType,
		t:            s.t,
		handler: func(r protoreflect.ProtoMessage) protoreflect.ProtoMessage {
			return dynamicpb.NewMessage(r.ProtoReflect().Descriptor())
		},
	}
	s.matchers = append(s.matchers, m)
	s.server.RegisterService(
		&grpc.ServiceDesc{
			ServiceName: serviceName,
			Methods: []grpc.MethodDesc{
				{
					MethodName: methodName,
					Handler:    s.newUnaryHandler(m),
				},
			},
		}, nil)

	return m
}

func (m *matcher) Response(message protoreflect.ProtoMessage) *matcher {
	prev := m.handler
	m.handler = func(r protoreflect.ProtoMessage) protoreflect.ProtoMessage {
		if prev != nil {
			prev(r)
		}
		return message
	}
	return m
}

// func (s *Server) Method(serviceName, methodName string, reqType protoreflect.ProtoMessage, respType protoreflect.ProtoMessage) *matcher {
// 	m := &matcher{
// 		t: s.t,
// 		handler: func(r protoreflect.ProtoMessage) protoreflect.ProtoMessage {
// 			return dynamicpb.NewMessage(r.ProtoReflect().Descriptor())
// 		},
// 	}
// 	s.matchers = append(s.matchers, m)
// 	return m
// }

func (s *Server) newUnaryHandler(m *matcher) func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		// in := dynamicpb.NewMessage(s.methodInfo.request.ProtoReflect().Descriptor())
		in := dynamicpb.NewMessage(m.requestType.ProtoReflect().Descriptor())
		if err := dec(in); err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.requests = append(s.requests, in)
		s.mu.Unlock()
		m.mu.Lock()
		m.requests = append(m.requests, in)
		m.mu.Unlock()
		fmt.Println("😀")
		return m.handler(in), nil
		// out := dynamicpb.NewMessage(m.requestType.ProtoReflect().Descriptor())
		// // out := dynamicpb.NewMessage(s.methodInfo.response.ProtoReflect().Descriptor())
		// if err := protojson.Unmarshal([]byte(`{"name":"hello"}`), out); err != nil {
		// 	return nil, err
		// }
		// return out, nil
	}
}
