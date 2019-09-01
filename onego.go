package main

import (
	"flag"
	"log"
	"os"
	"plugin"
	"os/exec"
	"encoding/gob"

	"github.com/valyala/fasthttp"
	"github.com/valyala/gorpc"
)

var (
	// Are these required args?
	mgmtaddr = flag.String("mgmtaddr", "127.0.0.1:8080", "Address to communicate with the onego daemon")
	daemonCmd = AddFlagSet("daemon", flag.ExitOnError)
	addr = daemonCmd.String("addr", "127.0.0.1:8000", "TCP address to listen to")
	addCmd = AddFlagSet("add", flag.ExitOnError)
	hostname = addCmd.String("hostname", "", "")
	srcfile = addCmd.String("srcfile", "", "")
)

var flagsets = map[string]*flag.FlagSet{}
func AddFlagSet(name string, errorHandling flag.ErrorHandling) *flag.FlagSet {
	fs := flag.NewFlagSet(name, errorHandling)
	flagsets[name] = fs
	return fs
}
func ParseFlagSets(args []string) {
	log.Println(args)
	if len(args) == 0 {
		log.Println("Expected a subcommand:")
		for k := range flagsets {
			log.Println("    " + k)
		}
        os.Exit(1)
	} else {
		flagset, ok := flagsets[os.Args[1]]
		if !ok {
			log.Println("Invalid subcommand. Try one of:")
			for k := range flagsets {
				log.Println("    " + k)
			}
			os.Exit(1)
		}
		flagset.Parse(os.Args[2:])
	}
}

func main() {
	gob.Register(AddReq{})
	flag.Parse()
	ParseFlagSets(flag.Args())
	if daemonCmd.Parsed() {
		serve(*addr, *mgmtaddr)
		// How do I actually daemonize?
	} else if addCmd.Parsed() {
		c := &gorpc.Client{ Addr: *mgmtaddr }
		c.Start()
		resp, err := c.Call(AddReq{Hostname: *hostname, Srcfile: *srcfile})
		if err != nil { log.Println("Failed to add:", err) }
		if resp != nil { log.Println("Failed to add:", resp) }
	} else {
		log.Println("Failed to parse.")
	}
}

type Host struct {
	handler func(ctx *fasthttp.RequestCtx, logger *log.Logger)
	logger *log.Logger
}

var seqid int
type AddReq struct {
	Hostname string
	Srcfile string
}

func serve(addr string, mgmt string) {
	var hosts = map[string]Host{}
	add := func(clientAddr string, req interface{}) interface{} {
		reqCast, ok := req.(AddReq)
		if !ok {
			return "request typecast failed"
		}
		hostname := reqCast.Hostname
		srcfile := reqCast.Srcfile
		// This is a shell injection vulnerability for 'hostname'
		built := "/tmp/onego/" + hostname + "." + string(seqid) + ".so"
		seqid++
		cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", built, srcfile)
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil { return "cmd.Run " + err.Error() }
		p, err := plugin.Open(built)
		if err != nil { return "plugin.Open " + err.Error() }
		sym, err := p.Lookup("handler")
		if err != nil { return "p.Lookup " + err.Error() }
		handler, ok := sym.(func(ctx *fasthttp.RequestCtx, logger *log.Logger));
		if !ok {
			return "symbol typecast failed"
		}
		logger := log.New(os.Stdout, hostname, log.Ldate | log.Ltime)
		hosts[hostname] = Host{handler: handler, logger: logger}
		return ""
	}

	h := func(ctx *fasthttp.RequestCtx) {
		defer func() {
			if r := recover(); r != nil {
				ctx.Error("An error occurred", fasthttp.StatusInternalServerError)
				log.Println("Recovered from panic in handler for", ctx.Host(), "with message", r)
			}
		}()
		h, ok := hosts[string(ctx.Host())]
		if ok {
			h.handler(ctx, h.logger)
		} else {
			ctx.Error("Route not found", fasthttp.StatusNotFound)
		}
	}

	s := &gorpc.Server{
		Addr: *mgmtaddr,
		Handler: add,
	}
	go s.Serve()

	err := fasthttp.ListenAndServe(addr, fasthttp.CompressHandler(h))
	if err != nil {
		log.Fatalf("Error in ListenAndServe: %s", err)
	}
}
