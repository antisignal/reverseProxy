package reverseProxy

import (
	"log"
	"net"
	"os"
	"strconv"
)

func main() {

	var ip net.IP
	var port int

	for i, arg := range os.Args {
		if i == 0 {
			continue
		}
		if arg == "--dest" {
			if len(os.Args) < i+2 {
				log.Fatalln("error: --dest passed without value!")
			}
			ip = net.ParseIP(os.Args[i+1])
			if ip == nil || ip.To4() == nil {
				log.Fatalln("error: --dest passed with invalid value!")
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

	var portStr string = ":" + strconv.Itoa(port)

	listener, err := net.Listen("tcp", portStr)
	if err != nil {
		log.Fatalln("failed to listen: ", err)
	}
	defer func() {
		err := listener.Close()
		if err != nil {
			log.Fatalln("failed to close listener: ", err)
		}
	}()
}
