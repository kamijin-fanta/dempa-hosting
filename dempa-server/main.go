package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kamijin-fanta/dempa-hosting/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
)

func main() {
	address := ":19003"
	listenPort, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalln(err)
	}
	server := grpc.NewServer(
		grpc.UnaryInterceptor(AuthServerInterceptor("../user-meta/__users.json")),
	)
	var dempaService moe_dempa_hosting.StaticHostingServer
	dempaService = &DempaServiceImpl{}
	moe_dempa_hosting.RegisterStaticHostingServer(server, dempaService)

	done := make(chan bool)
	go func() {
		fmt.Printf("started with %s\n\n", address)
		err = server.Serve(listenPort)
		if err != nil {
			panic(err)
		}
		done <- true
	}()

	httpService := &HttpService{
		service: dempaService.(*DempaServiceImpl),
	}
	go func() {
		httpAddress := ":8111"
		fmt.Printf("started with %s\n\n", httpAddress)
		err := http.ListenAndServe(httpAddress, httpService.ServerMux())
		if err != nil {
			panic(err)
		}
		done <- true
	}()
	<-done
}

type User struct {
	Token  string
	Secret string
}

func AuthServerInterceptor(userFile string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		md, _ := metadata.FromIncomingContext(ctx)
		token := md.Get("token")
		secret := md.Get("secret")

		if len(token) != 1 || len(secret) != 1 {
			return nil, status.Error(codes.Unauthenticated, "invalid token/secret")
		}

		file, err := os.Open(userFile)
		if err != nil {
			return nil, status.Error(codes.Internal, "cannot lookup user")
		}
		defer file.Close()
		users := []*User{}
		bytes, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, status.Error(codes.Internal, "cannot read users")
		}
		err = json.Unmarshal(bytes, &users)
		if err != nil {
			return nil, status.Error(codes.Internal, "cannot parse users")
		}

		found := false
		for _, user := range users {
			if token[0] == user.Token {
				if secret[0] == user.Secret {
					found = true
				} else {
					break
				}
			}
		}

		if found {
			return handler(ctx, req)
		} else {
			return nil, status.Error(codes.Unauthenticated, "invalid token/secret")
		}
	}
}
