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

	// arrange
	ts.Register("hello.GrpcTestService", "Hello", new(hello.HelloRequest), new(hello.HelloResponse)).Response(&hello.HelloResponse{
		Message: "Hello, world!",
	})
	ts.Start()

	// act
	ctx := context.Background()
	res, err := client.Hello(ctx, &hello.HelloRequest{Name: "qawatake"})

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != "Hello, world!" {
		t.Errorf("unexpected response: %s", res.Message)
	}
	{
		reqs := grpcmock.MapRequests[hello.HelloRequest](t, ts.Requests())
		if len(reqs) != 1 {
			t.Errorf("unexpected requests: %v", reqs)
		}
		got := reqs[0]
		if got.Name != "qawatake" {
			t.Errorf("unexpected request: %v", got)
		}
	}
}
