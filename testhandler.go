package main

import (
	"flag"
	"log"
	"os"

	"github.com/valyala/fasthttp"
)

var (
	addr  = flag.String("addr", "127.0.0.1:8080", "TCP address to listen to")
)

func main() {
	flag.Parse()
	fasthttp.ListenAndServe(*addr, func(ctx *fasthttp.RequestCtx) {
		logger := log.New(os.Stdout, "", log.LstdFlags)
		handler(ctx, logger)
	})
}

func handler(ctx *fasthttp.RequestCtx, logger *log.Logger) {
	ctx.SetBody([]byte("Hello http! " + string(ctx.URI().FullURI())))
	logger.Printf("hi logger %s", string(ctx.URI().FullURI()))
}
