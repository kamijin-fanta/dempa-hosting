package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

type HttpService struct {
	service *DempaServiceImpl
}

func (s *HttpService) ServerMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/", func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "not permitted", http.StatusNotAcceptable)
	})
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		fmt.Printf("HttpReq: request.Host: %#v\n", request.Host)

		pathArr := strings.Split(request.Host, ".")
		projectArr := strings.Split(pathArr[0], "--")
		projectId := projectArr[0]
		overrideRevision := ""
		if len(projectArr) == 2 {
			overrideRevision = projectArr[1]
		}

		meta, err := s.service.readUserMeta(projectId)
		fmt.Printf("Hello %#v %#v\n", meta, err)

		if err != nil {
			http.NotFound(writer, request)
			return
		}
		if meta.PublishRevision == "" || (len(projectArr) == 2 && overrideRevision == "") {
			http.NotFound(writer, request)
			return
		}
		if overrideRevision != "" {
			found := false
			for _, revision := range meta.Revisions {
				if revision.RevisionId == overrideRevision {
					found = true
					break
				}
			}
			if !found {
				http.NotFound(writer, request)
				return
			}
		}

		var rootPath string
		if overrideRevision == "" {
			rootPath = filepath.Join(s.service.UserContentDir, meta.ProjectId, meta.PublishRevision)
		} else {
			rootPath = filepath.Join(s.service.UserContentDir, meta.ProjectId, overrideRevision)
		}
		http.FileServer(http.Dir(rootPath)).ServeHTTP(writer, request)

		//fmt.Fprintf(writer, "Hello %#v %#v\n", meta, err)
	})
	return mux
}
