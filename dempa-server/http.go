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
		projectId := pathArr[0]

		meta, err := s.service.readUserMeta(projectId)
		fmt.Printf("Hello %#v %#v\n", meta, err)

		if err != nil {
			http.NotFound(writer, request)
			return
		}
		if meta.PublishRevision == "" {
			http.NotFound(writer, request)
			return
		}

		rootPath := filepath.Join("../user-data", meta.ProjectId, meta.PublishRevision)
		http.FileServer(http.Dir(rootPath)).ServeHTTP(writer, request)

		//fmt.Fprintf(writer, "Hello %#v %#v\n", meta, err)
	})
	return mux
}
