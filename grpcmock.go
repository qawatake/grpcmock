package grpcmock

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Server struct {
	matchers map[string]*matcher
	listener net.Listener
	server   *grpc.Server
	// tlsc              *tls.Config
	// cacert            []byte
	cc       *grpc.ClientConn
	requests []*dynamicpb.Message
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
	handler  handlerFunc
	requests []*dynamicpb.Message
	t        TB
	mu       sync.RWMutex
}

// type matchFunc func(r protoreflect.ProtoMessage) bool

type handlerFunc func(r protoreflect.ProtoMessage) protoreflect.ProtoMessage

func NewServer(t TB) *Server {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return &Server{
		server:   grpc.NewServer(),
		listener: lis,
		t:        t,
		matchers: make(map[string]*matcher),
	}
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

func (s *Server) Start() {
	go s.server.Serve(s.listener)
	// TODO: wait for ready
}

// Register registers a gRPC method to the internal gRPC server.
// Register must be called before Server.Start.
//
// Example:
// Register(ts, "/hello.GrpcTestService/Hello", hello.GrpcTestServiceClient.Hello)
func Register[R any, X, Y protoreflect.ProtoMessage](s *Server, fullMethodName string, method func(R, context.Context, X, ...grpc.CallOption) (Y, error)) *matcherx[X, Y] {
	s.t.Helper()
	var req X
	var res Y
	m := s.register(fullMethodName, req, res)
	return &matcherx[X, Y]{matcher: m}
}

// register registers a root matcher to the internal gRPC server.
//
// Example:
// fullMethodName: "/hello.GrpcTestService/Hello"
// reqType *hello.HelloRequest
// respType *hello.HelloResponse
func (s *Server) register(fullMethodName string, reqType protoreflect.ProtoMessage, respType protoreflect.ProtoMessage) *matcher {
	s.t.Helper()
	serviceName, methodName, err := parseFullMethodName(fullMethodName)
	if err != nil {
		s.t.Fatal(err)
		return nil
	}
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
	s.matchers[fullMethodName] = m
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

type matcherx[X, Y protoreflect.ProtoMessage] struct {
	matcher *matcher
}

// Response sets the response message for the matcher.
func (m *matcherx[X, Y]) Response(message Y) *matcherx[X, Y] {
	prev := m.matcher.handler
	m.matcher.handler = func(r protoreflect.ProtoMessage) protoreflect.ProtoMessage {
		if prev != nil {
			prev(r)
		}
		return message
	}
	return m
}

// Requests returns the requests received by the mock gRPC server.
func (s *Server) Requests() []*dynamicpb.Message {
	return s.requests
}

// Requests returns the requests received by the matcher.
func (m *matcherx[X, Y]) Requests() []X {
	ret := make([]X, 0, len(m.matcher.requests))
	for _, r := range m.matcher.requests {
		b, err := protojson.Marshal(r)
		if err != nil {
			m.matcher.t.Error(err)
			return nil
		}
		var x X
		xt := reflect.TypeFor[X]()
		x = reflect.New(xt.Elem()).Interface().(X)
		if err := protojson.Unmarshal(b, any(x).(protoreflect.ProtoMessage)); err != nil {
			m.matcher.t.Error(err)
			return nil
		}
		ret = append(ret, x)
	}
	return ret
}

func (s *Server) newUnaryHandler(m *matcher) func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
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
		return m.handler(in), nil
	}
}

// "/hello.GrpcTestService/Hello" -> ("hello.GrpcTestService", "Hello")
func parseFullMethodName(name string) (serviceName, methodName string, err error) {
	ss := strings.Split(name, "/")
	if len(ss) != 3 {
		return "", "", fmt.Errorf("%q is invalid full method name", name)
	}
	return ss[1], ss[2], nil
}
