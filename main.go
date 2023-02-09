package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/san-lab/capybara/httphandler"
)

//Parsing flags "ethport" and "host"
//Initializing EthRPC client
//TODO: Take care of the favicon.ico location
func main() {
	c := httphandler.Config{}

	rpc := flag.String("ethRPCAddress", "localhost:8545", "default RPC access point")
	tls := flag.Bool("tls", false, "https endpoint")
	httpPort := flag.String("httpPort", "8100", "http port")
	//c.MockMode = *flag.Bool("mockMode", false, "should mock http RPC client")
	dbugmode := flag.Bool("debugMode", false, "debug output?")
	dumpmode := flag.Bool("dumpRPC", false, "should dump RPC responses to files")
	//startWatchdog := flag.Bool("startWatchdog", false, "should a watchdog  be started")
	wauth := flag.Bool("withAuth", true, "should Basic Authentication be enabled")
	//httpsPortF := flag.Int("httpsPort", 0, "https port. tls not started if not provided. requires server.crt & server.key")
	flag.Parse()

	c.DumpRPC = *dumpmode
	c.DebugMode = *dbugmode
	c.RPCFirstEntry = *rpc
	c.BasicAuth = *wauth
	c.RPCTLS = *tls

	//c.StartWatchdog = *startWatchdog
	fmt.Println("Flags parsed. Starting the http server at", *httpPort)
	fmt.Println("The default RPC endpoint is", *rpc)

	//This is to graciously serve the ^C signal - allow all registered routines to clean up
	interruptChan := make(chan os.Signal)
	wg := &sync.WaitGroup{}
	signal.Notify(interruptChan, os.Interrupt)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	ctx = context.WithValue(ctx, "WaitGroup", wg)
	handler, err := httphandler.NewHttpHandler(c, ctx)
	if err != nil {
		panic(err)
	}
	http.HandleFunc("/favicon.ico", favIconHandler)
	//Beware! This config means that all the static images - also the ones called from the templates -
	// have to be addressed as "/static/*", regardless of the location of the template
	fs := http.FileServer(http.Dir("static"))
	http.HandleFunc("/static/", http.StripPrefix("/"+
		"static", fs).ServeHTTP)
	http.HandleFunc("/", handler.GetHandler(c.BasicAuth))
	srv := http.Server{Addr: "0.0.0.0:" + *httpPort}

	//This is to graciously serve the ^C signal - allow all registered routines to clean up
	go func() {
		select {
		case <-interruptChan:
			cancel()
			srv.Shutdown(context.TODO())
			return
		}
	}()

	log.Println(srv.ListenAndServe())
	wg.Wait()
}

func favIconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/build_32x32.png")
}
