package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var broadcastAddress = flag.String("broadcast_addr",
	"224.0.2.60", "address to broadcast to (generally leave default)")
var broadcastPort = flag.Uint("broadcast_port",
	4445, "port to broadcast to (generally leave default)")
var motd = flag.String("motd", "Hello Steve!", "MOTD to advertise")
var port = flag.Uint("port", 25565, "listening port of minecraft server")
var address = flag.String(
	"addr",
	"",
	"listening address of minecraft server (only needed for version < 1.6)",
)
var logTS = flag.Bool("log_ts", true, "include timestamp in log messages")

func init() {
	flag.Parse()

	*broadcastAddress += ":" + strconv.FormatUint(uint64(*broadcastPort), 10)
	*address += ":" + strconv.FormatUint(uint64(*port), 10)

	if *logTS {
		log.SetFlags(log.Lmicroseconds | log.Lshortfile | log.LUTC)
	} else {
		log.SetFlags(log.Lshortfile)
	}
}

func main() {
	log.Printf("broadcasting MOTD %q and address %q to %q",
		*motd, *address, *broadcastAddress)

	conn, err := net.DialTimeout("udp", *broadcastAddress, 15*time.Second)
	if err != nil {
		log.Fatalf("unable to connect to %q: %s", *broadcastAddress, err)
	}
	defer conn.Close()

	msg := fmt.Sprintf("[MOTD]%s[\\MOTD][AD]%s[\\AD]", *motd, *address)

	broadcastDone := make(chan struct{})
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		ticker := time.Tick(5 * time.Second)
		consecutiveFail := uint(0)
		for {
			select {
			case <-ticker:
				log.Println("tick")
				if err != nil {
					log.Printf("unable to broadcast msg: %s", err)
					if consecutiveFail > 5 {
						log.Println("unable to broadcast msg in 5 consecutive tries; exiting")
						close(broadcastDone)
						return
					}
					time.Sleep(5 << consecutiveFail * time.Second)
					consecutiveFail += 1
				} else {
					consecutiveFail = 0
				}
			case sig := <-sigs:
				log.Printf("exiting on signal %s\n", sig)
				close(broadcastDone)
				return
			}
		}
	}()

	<-broadcastDone
}
