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

	"github.com/cenkalti/backoff"
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
var verbose = flag.Bool("v", false, "verbose logging")

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

	msg := []byte(fmt.Sprintf("[MOTD]%s[\\MOTD][AD]%s[\\AD]", *motd, *address))

	broadcastDone := make(chan struct{})
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	backOff := backoff.NewExponentialBackOff()
	backOff.InitialInterval = 5 * time.Second
	backOff.Multiplier = 1.25
	backOff.MaxInterval = 5 * time.Second
	backOff.MaxElapsedTime = 30 * time.Second
	ticker := backoff.NewTicker(backOff)

	go func() {
		var err error
		for c := range ticker.C {
			if *verbose {
				log.Printf("c: %s\n", c)
			}
			_, err = conn.Write(msg)
			if err != nil {
				log.Printf("unable to broadcast msg: %s", err)
				continue
			}
			if *verbose {
				log.Println("success; resetting backoff")
			}
			backOff.Reset()
		}
		if err != nil {
			log.Fatalf("failed to broadcast msg within %s; exiting", backOff.MaxElapsedTime)
		}
		close(broadcastDone)
	}()

	go func() {
		sig := <-sigs
		log.Printf("exiting on signal %s\n", sig)
		ticker.Stop()
	}()

	<-broadcastDone
}
