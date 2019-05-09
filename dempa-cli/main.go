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
	app.Name = "dempa-cli"
	app.Usage = "static site hosting deploy tool"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "token",
			EnvVar: "DMP_TOKEN",
		},
		cli.StringFlag{
			Name:   "secret",
			EnvVar: "DMP_SECRET",
		},
		cli.StringFlag{
			Name:   "target",
			Usage:  "server name",
			EnvVar: "DMP_TARGET",
			Value:  "hosting.dempa.moe:19003",
		},
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
				cli.BoolFlag{
					Name:  "publish",
					Usage: "publish to production environment",
				},
				cli.StringFlag{
					Name:  "project",
					Usage: "Project ID",
				},
			},
			Action: func(c *cli.Context) error {
				projectId := c.String("project")
				fmt.Println("deploy start...")
				fmt.Printf("ProjectId: %s\n", projectId)
				if c.NArg() != 1 {
					log.Fatal("must specif deploy directory")
				}

				conn, service := MustNewService(
					c.GlobalString("target"),
					c.GlobalString("token"),
					c.GlobalString("secret"),
				)
				defer conn.Close()
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
			Name:      "init",
			Aliases:   []string{},
			Usage:     "init project",
			ArgsUsage: "PROJECT_ID",
			Action: func(c *cli.Context) error {
				fmt.Println("init project...")
				if c.NArg() != 1 {
					log.Fatal("must specif project id")
				}

				projectId := c.Args()[0]

				conn, service := MustNewService(
					c.GlobalString("target"),
					c.GlobalString("token"),
					c.GlobalString("secret"),
				)
				defer conn.Close()
				err := service.CreateProject(projectId)
				if err != nil {
					return err
				}
				fmt.Printf("success create project: %s\n", projectId)
				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func MustNewService(target, token, secret string) (*grpc.ClientConn, *ClientService) {
	conn, err := grpc.Dial(
		target,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(ClientAuthInterceptor(token, secret)),
	)
	if err != nil {
		log.Fatal("client connection error:", err)
	}
	client := moe_dempa_hosting.NewStaticHostingClient(conn)
	service := &ClientService{client: client}
	return conn, service
}
func ClientAuthInterceptor(token, secret string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md := metadata.New(map[string]string{
			"token":  token,
			"secret": secret,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)
		err := invoker(ctx, method, req, reply, cc, opts...)
		return err
	}
}

type ClientService struct {
	client moe_dempa_hosting.StaticHostingClient
}

func (s *ClientService) CreateProject(projectId string) error {
	createProjectReq := &moe_dempa_hosting.CreateProjectRequest{
		ProjectId: projectId,
	}
	_, err := s.client.CreateProject(context.TODO(), createProjectReq)
	if err != nil {
		return err
	}
	return nil
}
func (s *ClientService) CreateRevision(projectId string) (string, error) {
	revisionRequest := &moe_dempa_hosting.CreateRevisionRequest{
		ProjectId: projectId,
	}
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
