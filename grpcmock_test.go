package grpcmock_test

import (
	"context"
	"sync"
	"testing"

	"github.com/qawatake/grpcmock"
	"github.com/qawatake/grpcmock/testdata/hello"
	"github.com/qawatake/grpcmock/testdata/routeguide"
)

func TestServer(t *testing.T) {
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	ts.Register(hello.GrpcTestService_Hello_FullMethodName, new(hello.HelloRequest), new(hello.HelloResponse)).Response(&hello.HelloResponse{
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

func TestServerConcurrency(t *testing.T) {
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	ts.Register(hello.GrpcTestService_Hello_FullMethodName, new(hello.HelloRequest), new(hello.HelloResponse)).Response(&hello.HelloResponse{
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
		reqs := grpcmock.MapRequests[hello.HelloRequest](t, ts.Requests())
		if len(reqs) != 100 {
			t.Errorf("unexpected requests: %v", reqs)
		}
	}
}

func TestServerMethod(t *testing.T) {
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	helloClient := hello.NewGrpcTestServiceClient(conn)
	routeGuideClient := routeguide.NewRouteGuideClient(conn)

	// arrange
	ts.Register(hello.GrpcTestService_Hello_FullMethodName, new(hello.HelloRequest), new(hello.HelloResponse)).Response(&hello.HelloResponse{
		Message: "Hello, world!",
	})
	ts.Register(routeguide.RouteGuide_GetFeature_FullMethodName, new(routeguide.Point), new(routeguide.Feature)).Response(&routeguide.Feature{
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
		reqs := grpcmock.MapRequests[hello.HelloRequest](t, ts.Method(hello.GrpcTestService_Hello_FullMethodName).Requests())
		if len(reqs) != 1 {
			t.Errorf("unexpected requests: %v", reqs)
		}
		got := reqs[0]
		if got.Name != "qawatake" {
			t.Errorf("unexpected request: %v", got)
		}
	}
	{
		reqs := grpcmock.MapRequests[routeguide.Point](t, ts.Method(routeguide.RouteGuide_GetFeature_FullMethodName).Requests())
		if len(reqs) != 1 {
			t.Errorf("unexpected requests: %v", reqs)
		}
		got := reqs[0]
		if got.Latitude != 1 || got.Longitude != 2 {
			t.Errorf("unexpected request: %v", got)
		}
	}
}

func Register(t *testing.T) {
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	mx := grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
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
		reqs := mx.Requests()
		if len(reqs) != 1 {
			t.Errorf("unexpected requests: %v", reqs)
		}
		got := reqs[0]
		if got.Name != "qawatake" {
			t.Errorf("unexpected request: %v", got)
		}
	}
}
