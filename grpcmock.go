package grpcmock

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Server struct {
	// [fullMethodName][]*Matcher[I, O]
	matchers map[string]any
	// [ServiceName][MethodName]methodHandler
	methodHandler map[string]map[string]methodHandler
	listener      net.Listener
	server        *grpc.Server
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
		server:        grpc.NewServer(),
		listener:      lis,
		t:             t,
		matchers:      make(map[string]any),
		methodHandler: make(map[string]map[string]methodHandler),
	}
}

func (s *Server) ClientConn() *grpc.ClientConn {
	s.t.Helper()
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
	// we must defer registering services because RegisterService must be called at most once per service
	for serviceName, ms := range s.methodHandler {
		svc := serviceDesc(serviceName, ms)
		s.server.RegisterService(svc, nil)
	}
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
	serviceName, methodName, err := parseFullMethodName(fullMethodName)
	if err != nil {
		s.t.Fatal(err)
		return nil
	}
	m := &Matcher[I, O]{
		matchFunc: func(req *Request[I]) bool { return true },
		handler: func(r *Request[I]) (O, error) {
			var out O
			ot := reflect.TypeFor[O]()
			out = reflect.New(ot.Elem()).Interface().(O)
			return out, nil
		},
	}

	if s.methodHandler[serviceName] == nil {
		s.methodHandler[serviceName] = make(map[string]methodHandler)
	}
	s.methodHandler[serviceName][methodName] = newMethodHandler[I, O](s, fullMethodName)
	matchers, _ := s.matchers[fullMethodName].([]*Matcher[I, O])
	matchers = append(matchers, m)
	s.matchers[fullMethodName] = matchers
	return m
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

type Matcher[I, O protoreflect.ProtoMessage] struct {
	requests  []*Request[I]
	mu        sync.RWMutex
	matchFunc matchFunc[I, O]
	handler   handlerFunc[I, O]
}

type matchFunc[I, O protoreflect.ProtoMessage] func(req *Request[I]) bool

type handlerFunc[I, O protoreflect.ProtoMessage] func(r *Request[I]) (O, error)

// Match sets the request matcher.
func (m *Matcher[I, O]) Match(f func(I) bool) *Matcher[I, O] {
	prev := m.matchFunc
	m.matchFunc = func(req *Request[I]) bool {
		return prev(req) && f(req.Body)
	}
	return m
}

// MatchHeader sets the request header matcher.
func (m *Matcher[I, O]) MatchHeader(key, value string) *Matcher[I, O] {
	prev := m.matchFunc
	m.matchFunc = func(req *Request[I]) bool {
		return prev(req) && slices.Contains(req.Headers.Get(key), value)
	}
	return m
}

// Response sets the response body to be returned by the matcher.
func (m *Matcher[I, O]) Response(message O) *Matcher[I, O] {
	prev := m.handler
	m.handler = func(r *Request[I]) (O, error) {
		var err error
		if prev != nil {
			_, err = prev(r)
		}
		return message, err
	}
	return m
}

// Status sets the status to be returned by the matcher.
func (m *Matcher[I, O]) Status(s *status.Status) *Matcher[I, O] {
	prev := m.handler
	m.handler = func(r *Request[I]) (O, error) {
		if prev != nil {
			prev(r)
		}
		var out O
		return out, s.Err()
	}
	return m
}

// Handler sets the handler for the matcher.
func (m *Matcher[I, O]) Handler(f func(req I, h metadata.MD) (O, error)) *Matcher[I, O] {
	m.handler = func(r *Request[I]) (O, error) {
		return f(r.Body, r.Headers)
	}
	return m
}

type Request[I protoreflect.ProtoMessage] struct {
	Body    I
	Headers metadata.MD
}

// Requests returns the requests received by the matcher.
func (m *Matcher[I, O]) Requests() []*Request[I] {
	return m.requests
}

type methodHandler = func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error)

func newMethodHandler[I, O protoreflect.ProtoMessage](s *Server, fullMethodName string) methodHandler {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		var in I
		it := reflect.TypeFor[I]()
		in = reflect.New(it.Elem()).Interface().(I)
		if err := dec(in); err != nil {
			return nil, err
		}
		req := &Request[I]{Body: in, Headers: md}
		ms := s.matchers[fullMethodName].([]*Matcher[I, O])
		for _, m := range ms {
			if m.matchFunc(req) {
				m.mu.Lock()
				m.requests = append(m.requests, req)
				m.mu.Unlock()
				return m.handler(req)
			}
		}
		return nil, status.Error(codes.Unimplemented, "unimplemented")
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

func serviceDesc(serviceName string, methods map[string]methodHandler) *grpc.ServiceDesc {
	ms := make([]grpc.MethodDesc, 0, len(methods))
	for methodName, handler := range methods {
		ms = append(ms, grpc.MethodDesc{
			MethodName: methodName,
			Handler:    handler,
		})
	}
	return &grpc.ServiceDesc{
		ServiceName: serviceName,
		Methods:     ms,
	}
}
