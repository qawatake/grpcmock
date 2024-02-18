package grpcmock_test

import (
	"context"
	"sync"
	"testing"

	"github.com/qawatake/grpcmock"
	"github.com/qawatake/grpcmock/testdata/hello"
	"github.com/qawatake/grpcmock/testdata/routeguide"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestRegister(t *testing.T) {
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	helloRPC := grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
		Response(&hello.HelloResponse{
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
		reqs := helloRPC.Requests()
		if len(reqs) != 1 {
			t.Errorf("unexpected requests: %v", reqs)
		}
		got := reqs[0].Message
		if got.Name != "qawatake" {
			t.Errorf("unexpected request: %v", got)
		}
	}
}

func Register_concurrency(t *testing.T) {
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	helloRPC := grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
		Response(&hello.HelloResponse{
			Message: "Hello, world!",
		})
	ts.Start()

	// act
	ctx := context.Background()
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.Hello(ctx, &hello.HelloRequest{})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	// assert
	{
		reqs := helloRPC.Requests()
		if len(reqs) != 100 {
			t.Errorf("unexpected requests: %v", reqs)
		}
	}
}

func Register_multiple_methods(t *testing.T) {
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	helloClient := hello.NewGrpcTestServiceClient(conn)
	routeGuideClient := routeguide.NewRouteGuideClient(conn)

	// arrange
	helloRPC := grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
		Response(&hello.HelloResponse{
			Message: "Hello, world!",
		})
	featureRPC := grpcmock.Register(ts, routeguide.RouteGuide_GetFeature_FullMethodName, routeguide.RouteGuideClient.GetFeature).
		Response(&routeguide.Feature{
			Name: "test",
		})
	ts.Start()

	// act
	ctx := context.Background()
	helloRes, err1 := helloClient.Hello(ctx, &hello.HelloRequest{Name: "qawatake"})
	routeRes, err2 := routeGuideClient.GetFeature(ctx, &routeguide.Point{Latitude: 1, Longitude: 2})

	// assert
	if err1 != nil {
		t.Fatal(err2)
	}
	if helloRes.Message != "Hello, world!" {
		t.Errorf("unexpected response: %s", helloRes.Message)
	}
	if err2 != nil {
		t.Fatal(err2)
	}
	if routeRes.Name != "test" {
		t.Errorf("unexpected response: %s", routeRes.Name)
	}
	{
		reqs := helloRPC.Requests()
		if len(reqs) != 1 {
			t.Errorf("unexpected requests: %v", reqs)
		}
		got := reqs[0].Message
		if got.Name != "qawatake" {
			t.Errorf("unexpected request: %v", got)
		}
	}
	{
		reqs := featureRPC.Requests()
		if len(reqs) != 1 {
			t.Errorf("unexpected requests: %v", reqs)
		}
		got := reqs[0].Message
		if got.Latitude != 1 || got.Longitude != 2 {
			t.Errorf("unexpected request: %v", got)
		}
	}
}

func TestStatus(t *testing.T) {
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	helloRPC := grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
		Status(status.New(codes.Unknown, "unknown"))
	ts.Start()

	// act
	ctx := context.Background()
	res, err := client.Hello(ctx, &hello.HelloRequest{Name: "qawatake"})

	// assert
	if err == nil {
		t.Error("error expected but got nil")
	}
	if res != nil {
		t.Errorf("want nil, got %v", res)
	}
	{
		reqs := helloRPC.Requests()
		if len(reqs) != 1 {
			t.Errorf("unexpected requests: %v", reqs)
		}
		got := reqs[0].Message
		if got.Name != "qawatake" {
			t.Errorf("unexpected request: %v", got)
		}
	}
}

func TestHeaders(t *testing.T) {
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	helloRPC := grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello)
	ts.Start()

	// act
	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "key", "psj2buus")
	_, err := client.Hello(ctx, &hello.HelloRequest{})

	// assert
	if err != nil {
		t.Fatal(err)
	}
	{
		reqs := helloRPC.Requests()
		if len(reqs) != 1 {
			t.Errorf("unexpected requests: %v", reqs)
		}
		got := reqs[0].Headers.Get("key")
		if len(got) != 1 {
			t.Errorf("unexpected request: %v", got)
		}
		if got[0] != "psj2buus" {
			t.Errorf("unexpected request: %v", got)
		}
	}
}
