package grpcmock

import (
	"context"
	"reflect"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func Register[R any, X, Y protoreflect.ProtoMessage](s *Server, fullMethodName string, method func(R, context.Context, X, ...grpc.CallOption) (Y, error)) *matcherx[X, Y] {
	s.t.Helper()
	var req X
	var res Y
	m := s.Register(fullMethodName, req, res)
	return &matcherx[X, Y]{matcher: m}
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
