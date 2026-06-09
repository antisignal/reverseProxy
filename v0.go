package reverseProxy

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
)

func main() {

	var originIP net.IP
	var port int

	for i, arg := range os.Args {
		if i == 0 {
			continue
		}
		if arg == "--origin-ip" {
			if len(os.Args) < i+2 {
				log.Fatalln("error: --origin-ip passed without value!")
			}
			originIP = net.ParseIP(os.Args[i+1])
			if originIP == nil || originIP.To4() == nil {
				log.Fatalln("error: --origin-ip passed with invalid value!")
			}
		}
		if arg == "--port" {
			if len(os.Args) < i+2 {
				log.Fatalln("error: --port passed without value!")
			}
			var err error
			port, err = strconv.Atoi(os.Args[i+1])
			if err != nil {
				log.Fatalln("error: --port passed with invalid value!")
			}
		}
	}

	originStr, err := url.Parse("http://" + originIP.String() + ":" + strconv.Itoa(port))
	if err != nil {
		log.Fatalln("failed to parse origin URL:", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(originStr)

	http.Handle("/", proxy)
	log.Println("Listening on :" + strconv.Itoa(port))
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}
