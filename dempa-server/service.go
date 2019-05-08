package main

import (
	"context"
	"fmt"
	"github.com/kamijin-fanta/dempa-hosting/pb"
	"github.com/rs/xid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type DempaServiceImpl struct {
}

func (*DempaServiceImpl) Hello(ctx context.Context, req *moe_dempa_hosting.HelloRequest) (*moe_dempa_hosting.HelloResponse, error) {
	var res moe_dempa_hosting.HelloResponse
	res.Message = "welcome " + req.YourName
	return &res, nil
}

var projectRegex = regexp.MustCompile("^[a-z]+([-][a-z]+?)*$")

func (*DempaServiceImpl) CreateProject(ctx context.Context, req *moe_dempa_hosting.CreateProjectRequest) (*moe_dempa_hosting.CreateProjectResponse, error) {
	if !projectRegex.MatchString(req.ProjectId) {
		return nil, status.New(codes.InvalidArgument, "project_id is invalid").Err()
	}
	metaPath := filepath.Join("../user-meta", req.ProjectId+".json")
	if _, err := os.Stat(metaPath); os.IsExist(err) {
		return nil, status.New(codes.AlreadyExists, "Project already exists").Err()
	}

	os.Create(metaPath)

	res := moe_dempa_hosting.CreateProjectResponse{}
	return &res, nil
}

func (*DempaServiceImpl) CreateRevision(ctx context.Context, req *moe_dempa_hosting.CreateRevisionRequest) (*moe_dempa_hosting.CreateRevisionResponse, error) {
	guid := xid.New()
	res := moe_dempa_hosting.CreateRevisionResponse{
		RevisionId: guid.String(),
	}
	return &res, nil
}

func (*DempaServiceImpl) PutFile(stream moe_dempa_hosting.StaticHosting_PutFileServer) error {
	fmt.Printf("Recive File \n")
	projectId := ""
	revisionId := ""
	lastFilePath := ""

	//tmpFile, err := ioutil.TempFile("../tmp", "temp")
	//if err != nil {
	//	fmt.Printf("temp file open error %v\n", err)
	//	return err
	//}
	//defer os.Remove(tmpFile.Name())
	var file *os.File

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			// todo commit
			file.Close()
			res := &moe_dempa_hosting.PutFileResponse{}
			return stream.SendAndClose(res)
		}
		if err != nil {
			return err
		}

		if projectId != "" && projectId != in.ProjectId {
			return status.New(codes.InvalidArgument, "ProjectId").Err()
		}
		projectId = in.ProjectId
		if revisionId != "" && revisionId != in.RevisionId {
			return status.New(codes.InvalidArgument, "RevisionId").Err()
		}
		revisionId = in.RevisionId

		if lastFilePath != in.FilePath {
			if lastFilePath != "" {
				// todo commit
				file.Close()
				//tmpFile.Truncate(0)
				//tmpFile.Seek(0, 0)
			}
			fmt.Printf("  Project: %s Revition: %s Path: %s\n", projectId, revisionId, in.FilePath)
			// todo new file

			safePath := strings.ReplaceAll(in.FilePath, "..", "")
			writePath := filepath.Join("../user-data/", projectId, revisionId, safePath)
			fmt.Printf("  WritePath: %s\n", writePath)
			writeDir := filepath.Dir(writePath)
			os.MkdirAll(writeDir, os.ModeDir)
			file, err = os.Create(writePath)
			if err != nil {
				fmt.Printf("cannot open file")
				return status.New(codes.Internal, "Cannot Write File").Err()
			}
			file.Write(in.Chunk)

			// todo normalize path / directory traversal
			//tmpFile.Write(in.Chunk)
		} else {
			// todo send remain chunks
			//tmpFile.Write(in.Chunk)

			file.Write(in.Chunk)
		}

		lastFilePath = in.FilePath
	}
}

func (*DempaServiceImpl) RevisionClose(context.Context, *moe_dempa_hosting.RevisionCloseRequest) (*moe_dempa_hosting.RevisionCloseResponse, error) {
	res := moe_dempa_hosting.RevisionCloseResponse{}
	return &res, nil
}
