package main

import (
	"context"
	"fmt"
	"github.com/kamijin-fanta/dempa-hosting/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"log"
	"net"
)

func main() {
	address := ":19003"
	listenPort, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalln(err)
	}
	server := grpc.NewServer(
		grpc.UnaryInterceptor(AuthServerInterceptor()),
	)
	var dempaService moe_dempa_hosting.StaticHostingServer
	dempaService = &DempaServiceImpl{}
	moe_dempa_hosting.RegisterStaticHostingServer(server, dempaService)
	fmt.Printf("started with %s\n\n", address)
	err = server.Serve(listenPort)
	if err != nil {
		panic(err)
	}
}

func AuthServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		md, _ := metadata.FromIncomingContext(ctx)
		secrets := md.Get("secret")
		fmt.Printf("get secret in interceptor %#v\n", secrets)

		return handler(ctx, req)
	}
}
