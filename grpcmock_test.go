package grpcmock_test

import (
	"context"
	"testing"

	"github.com/qawatake/grpcmock"
	"github.com/qawatake/grpcmock/testdata/hello"
)

func TestServer(t *testing.T) {
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	ts.Register("hello.GrpcTestService", "Hello", new(hello.HelloRequest), new(hello.HelloResponse)).Response(&hello.HelloResponse{
		Message: "Hello, world!",
	})
	ts.Start()

	ctx := context.Background()
	res, err := client.Hello(ctx, &hello.HelloRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != "Hello, world!" {
		t.Errorf("unexpected response: %s", res.Message)
	}
}
