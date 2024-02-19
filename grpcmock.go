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
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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
	cc *grpc.ClientConn
	// requests []*Request[*dynamicpb.Message]
	// healthCheck       bool
	// disableReflection bool
	// status            serverStatus
	t TB
	// mu sync.RWMutex
}

var _ TB = (testing.TB)(nil)

type TB interface {
	Error(args ...any)
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Helper()
	Cleanup(func())
}

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

func (s *Server) Addr() string {
	return s.listener.Addr().String()
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
func Register[R any, I, O protoreflect.ProtoMessage](s *Server, fullMethodName string, method RPC[R, I, O]) *Matcher[I, O] {
	s.t.Helper()
	var in I
	var out O
	m := s.register(fullMethodName, in, out)
	return &Matcher[I, O]{matcher: m}
}

// RPC is a generic function type for gRPC methods.
//
// Example: hello.GrpcTestServiceClient.Hello
type RPC[R any, I, O protoreflect.ProtoMessage] func(R, context.Context, I, ...grpc.CallOption) (O, error)

// register registers a root matcher to the internal gRPC server.
//
// Example:
// fullMethodName: "/hello.GrpcTestService/Hello"
// reqType *hello.HelloRequest
// respType *hello.HelloResponse
func (s *Server) register(fullMethodName string, in protoreflect.ProtoMessage, out protoreflect.ProtoMessage) *matcher {
	s.t.Helper()
	serviceName, methodName, err := parseFullMethodName(fullMethodName)
	if err != nil {
		s.t.Fatal(err)
		return nil
	}
	m := &matcher{
		serviceName:  serviceName,
		methodName:   methodName,
		requestType:  in,
		responseType: out,
		t:            s.t,
		handler: func(r protoreflect.ProtoMessage) (protoreflect.ProtoMessage, error) {
			return dynamicpb.NewMessage(r.ProtoReflect().Descriptor()), nil
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

type Matcher[I, O protoreflect.ProtoMessage] struct {
	matcher *matcher
}

type matcher struct {
	serviceName  string
	methodName   string
	requestType  protoreflect.ProtoMessage
	responseType protoreflect.ProtoMessage
	// matchFuncs   []matchFunc
	handler  handlerFunc
	requests []*Request[*dynamicpb.Message]
	t        TB
	mu       sync.RWMutex
}

// type matchFunc func(r protoreflect.ProtoMessage) bool

type handlerFunc func(r protoreflect.ProtoMessage) (protoreflect.ProtoMessage, error)

// Response sets the response message for the matcher.
func (m *Matcher[I, O]) Response(message O) *Matcher[I, O] {
	prev := m.matcher.handler
	m.matcher.handler = func(r protoreflect.ProtoMessage) (protoreflect.ProtoMessage, error) {
		var err error
		if prev != nil {
			_, err = prev(r)
		}
		return message, err
	}
	return m
}

func (m *Matcher[I, O]) Status(s *status.Status) *Matcher[I, O] {
	prev := m.matcher.handler
	m.matcher.handler = func(r protoreflect.ProtoMessage) (protoreflect.ProtoMessage, error) {
		if prev != nil {
			prev(r)
		}
		return nil, s.Err()
	}
	return m
}

type Request[I protoreflect.ProtoMessage] struct {
	Body    I
	Headers metadata.MD
}

// Requests returns the requests received by the matcher.
func (m *Matcher[I, O]) Requests() []*Request[I] {
	ret := make([]*Request[I], 0, len(m.matcher.requests))
	for _, r := range m.matcher.requests {
		b, err := protojson.Marshal(r.Body)
		if err != nil {
			m.matcher.t.Error(err)
			return nil
		}
		var in I
		it := reflect.TypeFor[I]()
		in = reflect.New(it.Elem()).Interface().(I)
		if err := protojson.Unmarshal(b, any(in).(protoreflect.ProtoMessage)); err != nil {
			m.matcher.t.Error(err)
			return nil
		}
		ret = append(ret, &Request[I]{Body: in, Headers: r.Headers})
	}
	return ret
}

func (s *Server) newUnaryHandler(m *matcher) func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		in := dynamicpb.NewMessage(m.requestType.ProtoReflect().Descriptor())
		if err := dec(in); err != nil {
			return nil, err
		}
		m.mu.Lock()
		m.requests = append(m.requests, &Request[*dynamicpb.Message]{Body: in, Headers: md})
		m.mu.Unlock()
		return m.handler(in)
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
