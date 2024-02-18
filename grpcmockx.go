package grpcmock

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func Register[R any, X, Y protoreflect.ProtoMessage](s *Server, fullMethodName string, method func(R, context.Context, X, ...grpc.CallOption) (Y, error)) *matcherx[X, Y] {
	s.t.Helper()
	var req X
	var res Y

	serviceName, methodName, err := parseFullMethodName(fullMethodName)
	if err != nil {
		s.t.Fatal(err)
		return nil
	}
	mx := &matcherx[X, Y]{
		matcher: &matcher{
			serviceName:  serviceName,
			methodName:   methodName,
			requestType:  req,
			responseType: res,
			t:            s.t,
			handler: func(r protoreflect.ProtoMessage) protoreflect.ProtoMessage {
				return dynamicpb.NewMessage(r.ProtoReflect().Descriptor())
			},
		},
	}
	s.matchers[fullMethodName] = mx.matcher
	s.server.RegisterService(
		&grpc.ServiceDesc{
			ServiceName: serviceName,
			Methods: []grpc.MethodDesc{
				{
					MethodName: methodName,
					Handler:    s.newUnaryHandler(mx.matcher),
				},
			},
		}, nil)

	return mx
}

type matcherx[X, Y protoreflect.ProtoMessage] struct {
	matcher *matcher
}

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

func (m *matcherx[X, Y]) Requests() []X {
	ret := make([]X, 0, len(m.matcher.requests))
	for _, r := range m.matcher.requests {
		b, err := protojson.Marshal(r)
		if err != nil {
			m.matcher.t.Error(err)
			return nil
		}
		var x X
		x = (x.ProtoReflect()).(X)
		if err := protojson.Unmarshal(b, any(x).(protoreflect.ProtoMessage)); err != nil {
			m.matcher.t.Error(err)
			return nil
		}
		ret = append(ret, x)
	}
	return ret
}
