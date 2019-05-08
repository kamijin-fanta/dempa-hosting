package main

import (
	"context"
	"fmt"
	"github.com/kamijin-fanta/dempa-hosting/pb"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"io"
	"log"
	"os"
	"path/filepath"
)

func main() {
	app := cli.NewApp()
	app.Name = "boom"
	app.Usage = "make an explosive entrance"
	app.Action = func(c *cli.Context) error {
		fmt.Println("boom! I say!")
		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:  "login",
			Usage: "login service",
			Action: func(c *cli.Context) error {
				fmt.Println("login command")
				return nil
			},
		},
		{
			Name:      "deploy",
			Aliases:   []string{"d"},
			Usage:     "deploy files to service",
			ArgsUsage: "DIRECTORY",
			Flags: []cli.Flag{
				cli.BoolTFlag{
					Name:  "publish",
					Usage: "publish to production environment",
				},
			},
			Action: func(c *cli.Context) error {
				fmt.Println("deploy start...")
				projectId := "project-id"
				conn, service := MustNewService()
				defer conn.Close()
				if c.NArg() != 1 {
					log.Fatal("must specif deploy directory")
				}
				revision, err := service.CreateRevision(projectId)
				if err != nil {
					return err
				}
				err = service.UploadDirectory(projectId, revision, c.Args()[0])
				if err != nil {
					return err
				}
				err = service.CloseRevision(projectId, revision, c.Bool("publish"))
				if err != nil {
					return err
				}

				return nil
			},
		}, {
			Name:    "add",
			Aliases: []string{"a"},
			Usage:   "add a task to the list",
			Action: func(c *cli.Context) error {
				fmt.Println("add command")
				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func MustNewService() (*grpc.ClientConn, *ClientService) {
	conn, err := grpc.Dial(
		"127.0.0.1:19003",
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(ClientAuthInterceptor()),
	)
	if err != nil {
		log.Fatal("client connection error:", err)
	}
	client := moe_dempa_hosting.NewStaticHostingClient(conn)
	service := &ClientService{client: client}
	return conn, service
}
func ClientAuthInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md := metadata.Pairs("secret", "password")
		ctx = metadata.NewOutgoingContext(ctx, md)
		err := invoker(ctx, method, req, reply, cc, opts...)
		return err
	}
}

type ClientService struct {
	client moe_dempa_hosting.StaticHostingClient
}

func (s *ClientService) CreateRevision(projectId string) (string, error) {
	revisionRequest := &moe_dempa_hosting.CreateRevisionRequest{}
	revisionResponse, err := s.client.CreateRevision(context.TODO(), revisionRequest)
	if err != nil {
		return "", err
	}
	return revisionResponse.RevisionId, nil
}
func (s *ClientService) CloseRevision(projectId, revisionId string, publish bool) error {
	revisionRequest := &moe_dempa_hosting.RevisionCloseRequest{
		ProjectId:  projectId,
		RevisionId: revisionId,
		Publish:    publish,
	}
	_, err := s.client.RevisionClose(context.TODO(), revisionRequest)
	if err != nil {
		return err
	}
	return nil
}

func (s *ClientService) UploadDirectory(projectId, revisionId, targetDir string) error {
	stream, err := s.client.PutFile(context.TODO())
	if err != nil {
		return err
	}
	buffer := make([]byte, 1024*1024*1) // 1MiB
	err = filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		trimPath, _ := filepath.Rel(targetDir, path)
		fmt.Printf("  Sending: %s\n", trimPath)
		file, err := os.Open(path)
		for {
			readLen, err := file.Read(buffer)
			if err == io.EOF {
				break // end of file
			} else if err != nil {
				return err
			}
			msg := moe_dempa_hosting.PutFileRequest{
				ProjectId:     projectId,
				RevisionId:    revisionId,
				FilePath:      trimPath,
				TotalFileSize: int32(info.Size()),
				Chunk:         buffer[:readLen],
			}
			err = stream.Send(&msg)
			if err != nil {
				log.Fatal(err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}
	_, err = stream.CloseAndRecv()
	return err
}
