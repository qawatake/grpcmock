package grpcmock_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/qawatake/grpcmock"
	"github.com/qawatake/grpcmock/testdata/hello"
)

func Example() {
	t := &testing.T{} // provided by test

	ts := grpcmock.NewServer(t)
	client := hello.NewGrpcTestServiceClient(ts.ClientConn())

	helloRPC := grpcmock.Register(ts, "/hello.GrpcTestService/Hello", hello.GrpcTestServiceClient.Hello).
		Response(&hello.HelloResponse{
			Message: "Hello, world!",
		})

	// Start the server after registering the method.
	ts.Start()

	ctx := context.Background()
	res, _ := client.Hello(ctx, &hello.HelloRequest{Name: "qawatake"})

	fmt.Println("Response:", res.Message)
	fmt.Println("Request:", helloRPC.Requests()[0].Body.Name)
	// Output:
	// Response: Hello, world!
	// Request: qawatake
}
