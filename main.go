package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const broadcastTemplate = "[MOTD]%s[/MOTD][AD]%s[/AD]"

func init() {
	pflag.String("broadcast_addr", "224.0.2.60",
		"address to broadcast to (generally leave default)")
	pflag.Uint("broadcast_port", 4445,
		"port to broadcast to (generally leave default)")
	pflag.StringP("motd", "m", "Hello Steve!", "MOTD to advertise")
	pflag.UintP("port", "p", 25565, "listening port of minecraft server")
	pflag.StringP("addr", "a", "",
		"listening address of minecraft server (only needed for version < 1.6)")
	pflag.BoolP("log_ts", "l", true, "include timestamp in log messages")
	pflag.UintP("verbose", "v", 0, "verbose logging")

	pflag.Parse()

	viper.SetEnvPrefix("MCLB")
	viper.AutomaticEnv()
	viper.BindPFlags(pflag.CommandLine)

	if viper.GetBool("log_ts") {
		log.SetFlags(log.Lmicroseconds | log.Lshortfile | log.LUTC)
	} else {
		log.SetFlags(log.Lshortfile)
	}
}

func combineAddressPort(addr string, port uint) string {
	return addr + ":" + strconv.FormatUint(uint64(port), 10)
}

func main() {
	motd := viper.GetString("motd")
	var addr string
	if viper.IsSet("addr") {
		addr = combineAddressPort(viper.GetString("addr"), viper.GetUint("port"))
	} else {
		addr = strconv.ParseUint(uint64(viper.GetUint("port")), 10)
	}
	broadcastAddr := combineAddressPort(
		viper.GetString("broadcast_addr"), viper.GetUint("broadcast_port"))

	log.Printf("broadcasting MOTD %q and address %q to %q",
		motd, addr, broadcastAddr)

	conn, err := net.DialTimeout("udp", broadcastAddr, 15*time.Second)
	if err != nil {
		log.Fatalf("unable to connect to %q: %s", broadcastAddr, err)
	}
	defer conn.Close()

	msg := []byte(fmt.Sprintf(broadcastTemplate, motd, addr))

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
			if viper.GetUint("verbose") > 0 {
				log.Printf("c: %s\n", c)
			}
			_, err = conn.Write(msg)
			if err != nil {
				log.Printf("unable to broadcast msg: %s", err)
				continue
			}
			if viper.GetUint("verbose") > 0 {
				log.Println("success; resetting backoff")
			}
			backOff.Reset()
		}
		if err != nil {
			log.Fatalf("failed to broadcast msg within %s; exiting",
				backOff.MaxElapsedTime)
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
