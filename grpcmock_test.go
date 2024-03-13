package grpcmock_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/qawatake/grpcmock"
	"github.com/qawatake/grpcmock/testdata/gen/hello"
	"github.com/qawatake/grpcmock/testdata/gen/routeguide"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestMatcher_Match(t *testing.T) {
	t.Parallel()
	tests := []struct {
		testName string
		name     string
		wantMsg  string
		wantErr  bool
	}{
		{
			testName: "matched",
			name:     "qawatake",
			wantMsg:  "Hello, world!",
			wantErr:  false,
		},
		{
			testName: "not matched",
			name:     "other",
			wantMsg:  "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()

			ts := grpcmock.NewServer(t)
			conn := ts.ClientConn()
			client := hello.NewGrpcTestServiceClient(conn)

			// arrange
			grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
				Match(func(req *hello.HelloRequest) bool {
					return req.Name == "qawatake"
				}).
				Response(&hello.HelloResponse{
					Message: "Hello, world!",
				})
			ts.Start()

			// act
			ctx := context.Background()
			res, err := client.Hello(ctx, &hello.HelloRequest{Name: tt.name})

			// assert
			if tt.wantErr {
				if err == nil {
					t.Error("error expected but got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if res.Message != tt.wantMsg {
				t.Errorf("unexpected response: %s", res.Message)
			}
		})
	}
}

func TestMatcher_Match_multiple(t *testing.T) {
	t.Parallel()

	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
		Match(func(req *hello.HelloRequest) bool {
			return req.Name == "qawatake"
		}).
		Response(&hello.HelloResponse{
			Message: "Hello, world!",
		})
	grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
		Match(func(req *hello.HelloRequest) bool {
			return req.Name == "other"
		}).
		Response(&hello.HelloResponse{
			Message: "Hello, other!",
		})
	ts.Start()

	// act
	ctx := context.Background()
	res1, err1 := client.Hello(ctx, &hello.HelloRequest{Name: "qawatake"})
	res2, err2 := client.Hello(ctx, &hello.HelloRequest{Name: "other"})

	// assert
	{
		if err1 != nil {
			t.Fatal(err1)
		}
		if res1.Message != "Hello, world!" {
			t.Errorf("unexpected response: %s", res1.Message)
		}
	}
	{
		if err2 != nil {
			t.Fatal(err2)
		}
		if res2.Message != "Hello, other!" {
			t.Errorf("unexpected response: %s", res2.Message)
		}
	}
}

func TestMatcher_MatchHeader(t *testing.T) {
	t.Parallel()
	tests := []struct {
		testName    string
		headerValue string
		wantMsg     string
		wantErr     bool
	}{
		{
			testName:    "matched",
			headerValue: "true",
			wantMsg:     "Hello, world!",
			wantErr:     false,
		},
		{
			testName:    "not matched",
			headerValue: "false",
			wantMsg:     "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()

			ts := grpcmock.NewServer(t)
			conn := ts.ClientConn()
			client := hello.NewGrpcTestServiceClient(conn)

			// arrange
			grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
				MatchHeader("x-foo", "true").
				Response(&hello.HelloResponse{
					Message: "Hello, world!",
				})
			ts.Start()

			// act
			ctx := context.Background()
			ctx = metadata.AppendToOutgoingContext(ctx, "x-foo", tt.headerValue)
			res, err := client.Hello(ctx, &hello.HelloRequest{})

			// assert
			if tt.wantErr {
				if err == nil {
					t.Error("error expected but got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if res.Message != tt.wantMsg {
				t.Errorf("unexpected response: %s", res.Message)
			}
		})
	}
}

func TestMatcher_Response(t *testing.T) {
	t.Parallel()
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
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
}

func TestMatcher_Status(t *testing.T) {
	t.Parallel()
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
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
}

func TestMatcher_Handler(t *testing.T) {
	t.Parallel()
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
		Handler(func(r *hello.HelloRequest, md metadata.MD) (*hello.HelloResponse, error) {
			name := r.Name
			count, err := strconv.Atoi(md.Get("x-count")[0])
			if err != nil {
				t.Error(err)
				return nil, err
			}
			return &hello.HelloResponse{
				Message: fmt.Sprintf("Hello, %s%s", name, strings.Repeat("!", count)),
			}, nil
		})
	ts.Start()

	// act
	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "x-count", "3")
	res, err := client.Hello(ctx, &hello.HelloRequest{Name: "qawatake"})

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != "Hello, qawatake!!!" {
		t.Errorf("unexpected response: %s", res.Message)
	}
}

func TestMatcher_Requests(t *testing.T) {
	t.Parallel()
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	helloRPC := grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello)
	ts.Start()

	// act
	ctx := context.Background()
	client.Hello(ctx, &hello.HelloRequest{Name: "qawatake"})

	// assert
	reqs := helloRPC.Requests()
	if len(reqs) != 1 {
		t.Errorf("unexpected requests: %v", reqs)
	}
	got := reqs[0].Body
	if got.Name != "qawatake" {
		t.Errorf("unexpected request: %v", got)
	}
}

func TestMatcher_Requests_concurrency(t *testing.T) {
	t.Parallel()
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	helloRPC := grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello)
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

func TestMatcher_Requests_multiple_methods(t *testing.T) {
	t.Parallel()
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
		got := reqs[0].Body
		if got.Name != "qawatake" {
			t.Errorf("unexpected request: %v", got)
		}
	}
	{
		reqs := featureRPC.Requests()
		if len(reqs) != 1 {
			t.Errorf("unexpected requests: %v", reqs)
		}
		got := reqs[0].Body
		if got.Latitude != 1 || got.Longitude != 2 {
			t.Errorf("unexpected request: %v", got)
		}
	}
}

func TestRequest_Headers(t *testing.T) {
	ts := grpcmock.NewServer(t)
	conn := ts.ClientConn()
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	helloRPC := grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello)
	ts.Start()

	// act
	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "key", "psj2buus")
	client.Hello(ctx, &hello.HelloRequest{})

	// assert
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

func TestServer_Addr(t *testing.T) {
	t.Parallel()
	ts := grpcmock.NewServer(t)
	addr := ts.Addr()
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	client := hello.NewGrpcTestServiceClient(conn)

	// arrange
	grpcmock.Register(ts, hello.GrpcTestService_Hello_FullMethodName, hello.GrpcTestServiceClient.Hello).
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
}
