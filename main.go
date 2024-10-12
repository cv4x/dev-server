/*
dev-srv hosts multiple static file directories, each on its own port.

https://github.com/claire-west/dev-server

Usage:

	dev-srv
	dev-srv [<file>]

Define a set of services in a file in the following format:

	8080=/home/user/git/myfirstproject
	9090=../coolwebthing/public

If no argument is provided, <file> will default to "services".
*/
package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

type Service struct {
	path string
	port int
}

const Y = "\033[33m"
const B = "\033[34m"
const C = "\033[36m"
const R = "\033[0m"

func main() {
	var serviceFile string
	if len(os.Args) < 2 || len(os.Args[1]) == 0 {
		executable, err := os.Executable()
	    if err != nil {
	        panic(err)
	    }
		serviceFile = filepath.Dir(executable) + "/services"
	} else {
		serviceFile = os.Args[1]
	}

	var waitGroup sync.WaitGroup

	services := readServices(serviceFile)
	waitGroup.Add(len(services))

	var servers []*http.Server
	for _, service := range services {
		server := startService(service)
		server.RegisterOnShutdown(func() {
			log.Printf("%s%d%s → %sStopped%s\n", Y, service.port, R, C, R)
			waitGroup.Done()
		})
		servers = append(servers, server)
	}

	interrupted := make(chan os.Signal, 1)
    signal.Notify(interrupted, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
    <-interrupted

    ctx := context.Background()
    for _, server := range servers {
    	server.Shutdown(ctx)
    }

	waitGroup.Wait()
}

func readServices(fileName string) []Service {
	read, err := os.Open(fileName)
	if err != nil { panic(err) }

	scanner := bufio.NewScanner(read)

	var services []Service
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "=")
		port, err := strconv.Atoi(parts[0])
		if err != nil { panic(err) }

		services = append(services, Service{
			path: parts[1],
			port: port,
		})
	}

	return services
}

type DirWithHtmlFallback struct {
	dir http.Dir
}

func (d DirWithHtmlFallback) Open (name string) (http.File, error) {
	f, err := d.dir.Open(name)
	if os.IsNotExist(err) && filepath.Ext(name) == "" {
		f, err = d.dir.Open(name + ".html")
	}
	return f, err
}

func startService(service Service) *http.Server {
	dir := DirWithHtmlFallback{http.Dir(service.path)}
    fsHandler := http.FileServer(dir)
    handler := http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
   		log.Printf("%s%s%s%s%s", C, service.path, B, req.URL.Path, R)

     	resp.Header().Add("access-control-allow-origin", "*")
   		fsHandler.ServeHTTP(resp, req)
    })

	server := &http.Server{
		Addr: ":" + strconv.Itoa(service.port),
		Handler: handler,
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	log.Printf("%s%d%s → %s%s%s\n", Y, service.port, R, C, service.path, R)

    return server
}
