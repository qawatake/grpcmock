# grpcmock

[![Go Reference](https://pkg.go.dev/badge/github.com/qawatake/grpcmock.svg)](https://pkg.go.dev/github.com/qawatake/grpcmock)
[![test](https://github.com/qawatake/grpcmock/actions/workflows/test.yaml/badge.svg)](https://github.com/qawatake/grpcmock/actions/workflows/test.yaml)
[![codecov](https://codecov.io/gh/qawatake/grpcmock/graph/badge.svg?token=iBMN98cHlc)](https://codecov.io/gh/qawatake/grpcmock)

grpcmock provides a mock gRPC server from a generated gRPC client code.

```go
func TestClient(t *testing.T) {
  ts := grpcmock.NewServer(t)
  conn := ts.ClientConn()
  client := hello.NewGrpcTestServiceClient(conn)

  // arrange
  helloRPC := grpcmock.Register(ts, "/hello.GrpcTestService/Hello", hello.GrpcTestServiceClient.Hello).
    Response(&hello.HelloResponse{
      Message: "Hello, world!",
    })
  ts.Start()

  // act
  ctx := context.Background()
  res, _ := client.Hello(ctx, &hello.HelloRequest{Name: "qawatake"})

  // assert
  if res.Message != "Hello, world!" {
    t.Errorf("unexpected response: %s", res.Message)
  }
  {
    got := helloRPC.Requests()[0].Body
    if got.Name != "qawatake" {
      t.Errorf("unexpected request: %v", got)
    }
  }
}
```

## Limitations

- [ ] General matching is not supported.
- [ ] Header matcher is not supported.
- [ ] Only unary RPCs are supported.

## References

- [k1LoW/grpcstub]: provides a stub gRPC server from protobuf files (not from generated gRPC client code)

<!-- links -->

[k1LoW/grpcstub]: https://github.com/k1LoW/grpcstub
