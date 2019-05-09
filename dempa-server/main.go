package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kamijin-fanta/dempa-hosting/pb"
	"github.com/urfave/cli"
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
	app := cli.NewApp()
	app.Name = "dempa-server"
	app.Usage = "static site hosting service"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "token",
			EnvVar: "DMP_TOKEN",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "server",
			Usage: "start server",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:   "grpc-port",
					EnvVar: "DMP_GRPC_PORT",
					Value:  19003,
				},
				cli.IntFlag{
					Name:   "http-content-port",
					EnvVar: "DMP_HTTP_CONTENT_PORT",
					Value:  19004,
				},
				cli.StringFlag{
					Name:   "users-json-path",
					EnvVar: "DMP_USERS_JSON_PATH",
					Value:  "../data/user-meta/__users.json",
				},
				cli.StringFlag{
					Name:   "user-meta-dir",
					EnvVar: "DMP_USER_META_DIR",
					Value:  "../data/user-meta/",
				},
				cli.StringFlag{
					Name:   "user-content-dir",
					EnvVar: "DMP_USER_CONTENT_DIR",
					Value:  "../data/user-content/",
				},
			},
			Action: func(c *cli.Context) error {
				grpcAddress := fmt.Sprintf(":%d", c.Int("grpc-port"))
				listenPort, err := net.Listen("tcp", grpcAddress)
				if err != nil {
					log.Fatalln(err)
				}
				server := grpc.NewServer(
					grpc.UnaryInterceptor(AuthServerInterceptor(c.String("users-json-path"))),
				)
				var dempaService moe_dempa_hosting.StaticHostingServer
				dempaService = NewDempaService(c.String("user-meta-dir"), c.String("user-content-dir"))
				moe_dempa_hosting.RegisterStaticHostingServer(server, dempaService)

				done := make(chan bool)
				go func() {
					fmt.Printf("gRPC API server started with %s\n\n", grpcAddress)
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
					httpAddress := fmt.Sprintf(":%s", c.String("http-content-port"))
					fmt.Printf("HTTP content server started with %s\n\n", httpAddress)
					err := http.ListenAndServe(httpAddress, httpService.ServerMux())
					if err != nil {
						panic(err)
					}
					done <- true
				}()
				<-done

				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
	return
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
