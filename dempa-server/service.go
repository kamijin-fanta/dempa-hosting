package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kamijin-fanta/dempa-hosting/pb"
	"github.com/rs/xid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type UserMeta struct {
	ProjectId       string
	PublishRevision string
	Revisions       []*UserMetaRevision
}
type UserMetaRevision struct {
	RevisionId string
	Timestamp  time.Time
	Closed     bool
}

type DempaServiceImpl struct {
}

func (*DempaServiceImpl) Hello(ctx context.Context, req *moe_dempa_hosting.HelloRequest) (*moe_dempa_hosting.HelloResponse, error) {
	var res moe_dempa_hosting.HelloResponse
	res.Message = "welcome " + req.YourName
	return &res, nil
}

var projectRegex = regexp.MustCompile("^[a-z]+([-][a-z]+?)*$")

func (*DempaServiceImpl) writeUserMeta(meta *UserMeta) error {
	metaPath := filepath.Join("../user-meta", meta.ProjectId+".json")
	file, err := os.Create(metaPath)
	defer file.Close()
	if err != nil {
		return err
	}
	bytes, _ := json.Marshal(meta)
	file.Write(bytes)
	return nil
}
func (*DempaServiceImpl) readUserMeta(projectId string) (*UserMeta, error) {
	safeProjectId := strings.ReplaceAll(projectId, ".", "")
	metaPath := filepath.Join("../user-meta", safeProjectId+".json")
	file, err := os.Open(metaPath)
	defer file.Close()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.New(codes.NotFound, "not found meta").Err()
		}
		return nil, err
	}
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	meta := &UserMeta{}
	err = json.Unmarshal(bytes, meta)
	if err != nil {
		return nil, err
	}

	return meta, nil
}

func (s *DempaServiceImpl) CreateProject(ctx context.Context, req *moe_dempa_hosting.CreateProjectRequest) (*moe_dempa_hosting.CreateProjectResponse, error) {
	if !projectRegex.MatchString(req.ProjectId) {
		return nil, status.New(codes.InvalidArgument, "project_id is invalid").Err()
	}
	metaPath := filepath.Join("../user-meta", req.ProjectId+".json")
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		return nil, status.New(codes.AlreadyExists, "Project already exists").Err()
	}

	meta := UserMeta{
		ProjectId: req.ProjectId,
	}
	s.writeUserMeta(&meta)

	res := moe_dempa_hosting.CreateProjectResponse{}
	return &res, nil
}

func (s *DempaServiceImpl) CreateRevision(ctx context.Context, req *moe_dempa_hosting.CreateRevisionRequest) (*moe_dempa_hosting.CreateRevisionResponse, error) {
	guid := xid.New()
	res := moe_dempa_hosting.CreateRevisionResponse{
		RevisionId: guid.String(),
	}
	fmt.Printf("CreateRevison %#v\n", req)
	meta, err := s.readUserMeta(req.ProjectId)
	if err != nil {
		return nil, err
	}
	revision := &UserMetaRevision{
		RevisionId: res.RevisionId,
		Closed:     false,
		Timestamp:  time.Now(),
	}
	if meta.Revisions == nil {
		meta.Revisions = []*UserMetaRevision{}
	}
	meta.Revisions = append(meta.Revisions, revision)
	err = s.writeUserMeta(meta)
	if err != nil {
		return nil, err
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

func (s *DempaServiceImpl) RevisionClose(ctx context.Context, req *moe_dempa_hosting.RevisionCloseRequest) (*moe_dempa_hosting.RevisionCloseResponse, error) {
	res := moe_dempa_hosting.RevisionCloseResponse{}

	meta, err := s.readUserMeta(req.ProjectId)
	if err != nil {
		return nil, err
	}
	found := false
	for _, rev := range meta.Revisions {
		if rev.RevisionId == req.RevisionId {
			rev.Closed = true
			found = true
			break
		}
	}
	if !found {
		return nil, status.New(codes.NotFound, "not found revision").Err()
	}
	if req.Publish {
		meta.PublishRevision = req.RevisionId
	}
	err = s.writeUserMeta(meta)
	if err != nil {
		return nil, err
	}

	return &res, nil
}
