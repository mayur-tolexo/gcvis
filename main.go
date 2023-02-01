// gcvis is a tool to assist you visualising the operation of
// the go runtime garbage collector.
//
// usage:
//
//	gcvis program [arguments]...
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/mayur-tolexo/gcvis/exec"
	"github.com/mayur-tolexo/gcvis/graph"
	"github.com/mayur-tolexo/gcvis/server"
	"github.com/pkg/browser"

	"golang.org/x/crypto/ssh/terminal"
)

var iface = flag.String("i", "127.0.0.1", "specify interface to use. defaults to 127.0.0.1.")
var port = flag.String("p", "0", "specify port to use.")
var filename = flag.String("f", "", "filename of the log file")
var openBrowser = flag.Bool("o", true, "automatically open browser")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s: command <args>...\n", os.Args[0])
		flag.PrintDefaults()
	}

	var pipeRead io.ReadCloser
	var subcommand *exec.SubCommand

	flag.Parse()
	if *filename != "" {
		file, err := os.Open(*filename)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer file.Close()
		pipeRead = file
	} else {
		if len(flag.Args()) < 1 {
			if terminal.IsTerminal(int(os.Stdin.Fd())) {
				flag.Usage()
				return
			}
			pipeRead = os.Stdin
		} else {
			subcommand = exec.NewSubCommand(flag.Args())
			pipeRead = subcommand.PipeRead
			go subcommand.Run()
		}
	}

	parser := graph.NewParser(pipeRead)

	title := strings.Join(flag.Args(), " ")
	if len(title) == 0 {
		title = fmt.Sprintf("%s:%s", *iface, *port)
	}

	gcvisGraph := graph.NewGraph(title, graph.GCVIS_TMPL)
	server := server.NewHttpServer(*iface, *port, &gcvisGraph)

	go parser.Run()
	go server.Start()

	url := server.Url()

	if *openBrowser {
		log.Printf("opening browser window, if this fails, navigate to %s", url)
		browser.OpenURL(url)
	} else {
		log.Printf("server started on %s", url)
	}

	for {
		select {
		case gcTrace := <-parser.GcChan:
			gcvisGraph.AddGCTraceGraphPoint(gcTrace)
		case scvgTrace := <-parser.ScvgChan:
			gcvisGraph.AddScavengerGraphPoint(scvgTrace)
		case output := <-parser.NoMatchChan:
			fmt.Fprintln(os.Stderr, output)
		case <-parser.Done:
			if *filename != "" {
				if parser.Err != nil {
					fmt.Fprintf(os.Stderr, parser.Err.Error())
					os.Exit(1)
				}

				os.Exit(0)
			}
		}
	}

	if subcommand != nil && subcommand.Err() != nil {
		fmt.Fprintf(os.Stderr, subcommand.Err().Error())
		os.Exit(1)
	}

	os.Exit(0)
}
