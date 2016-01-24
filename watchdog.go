package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

// messages for communicating between goroutines
type Message struct {
	Sender    *net.UDPAddr
	Msgid     uint32
	Timestamp time.Time
}

func (m *Message) String() string {
	return fmt.Sprintf("Message{cnt: %v, sender: %v, timestamp:\"%v\"}",
		m.Msgid, m.Sender, m.Timestamp)
}

// reading configuration from json
type Config struct {
	Interval int      // how often we send i-am-alive packets
	Timeout  int      // how long we wait before flagging a node as down
	Port     int      // the UDP-port to open for listening
	Peers    []string // list of UDP-addresse to send packets to
}

func (c *Config) String() string {
	return fmt.Sprintf("Config{interval: %v, timeout: %v, port: %v, peers: %v}",
		c.Interval, c.Timeout, c.Port, c.Peers)
}

func readConfig(filename string) (config *Config, err error) {
	fmt.Printf("Reaing configuration from %v\n", filename)
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer func() {
		cErr := file.Close()
		if err == nil {
			err = cErr
		}
	}()

	dec := json.NewDecoder(file)
	config = &Config{}
	err = dec.Decode(config)

	return
}

// start server
func server(udpConn *net.UDPConn, c chan *Message) {
	fmt.Println("Starting server")
	buffer := make([]byte, 1500)
	i := 0
	for {
		cnt, peerAddr, err := udpConn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading from udp port, %v\n", err)
		} else {
			m := &Message{Sender: peerAddr}
			m.Msgid = binary.BigEndian.Uint32(buffer[0:])
			m.Timestamp = time.Unix(int64(binary.BigEndian.Uint64(buffer[4:])), 0)
			c <- m
		}
		if cnt == 1 {
			close(c)
			return
		}
		i++
	}
}

// sending ping to all peers
func ping(udpConn *net.UDPConn, interval time.Duration, peers []*net.UDPAddr) {
	fmt.Println("Starting ping")
	msg := make([]byte, 1500)
	msg[0] = 1
	var i int32 = 0
	// from https://golang.org/pkg/time/#example_Tick
	c := time.Tick(interval)
	for now := range c {
		binary.BigEndian.PutUint32(msg[0:], uint32(i))
		binary.BigEndian.PutUint64(msg[4:], uint64(now.Unix()))
		for _, peer := range peers {
			if peer != nil {
				_, err := udpConn.WriteToUDP(msg[0:12], peer)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error sending UDP-packet to %v: %v\n", peer, err)
				}
			}
		}
		i++
	}
}

func listen(c chan *Message, timeout time.Duration) {
	fmt.Printf("Starting listener, timeout=%v\n", timeout)
	timeoutTicker := time.Tick(5 * time.Second)
	peers := make(map[string]time.Time)
	for {
		select {
		case msg := <-c:
			fmt.Printf("Recieved %v\n", msg)
			peers[fmt.Sprintf("%v", msg.Sender)] = time.Now()
		case now := <-timeoutTicker:
			fmt.Printf("Time to check for timeouts...\n")
			for peer, lasttime := range peers {
				interval := now.Sub(lasttime)
				if interval > timeout {
					fmt.Printf("Timeout from %v: %v\n", peer, interval)
				}
			}
		}
	}
}

func main() {
	cfgfile := "cfg.json"
	if len(os.Args) > 1 {
		cfgfile = os.Args[1]
	}

	config, err := readConfig(cfgfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading configuration: %v\n", err)
		return
	}

	fmt.Printf("configuration: %v\n", config)

	fmt.Printf("Start\n")
	serverAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%v", config.Port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving UDP-address: %v\n", err)
		return
	}

	udpConn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening %v for listening: %v\n", serverAddr, err)
		return
	}

	peers := make([]*net.UDPAddr, len(config.Peers))
	var i int = 0
	for _, peerAddr := range config.Peers {
		fmt.Printf("Adding peer %v\n", peerAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving UDP-address %v: %v\n", peerAddr, err)
		} else {
			peers[i], err = net.ResolveUDPAddr("udp", peerAddr)
			i++
		}
	}

	if i == 0 {
		fmt.Fprintf(os.Stderr, "no peers configured!\n")
		return
	}

	c := make(chan *Message, 16)

	go server(udpConn, c)

	go ping(udpConn, time.Duration(config.Interval)*time.Second, peers)

	listen(c, time.Duration(config.Timeout)*time.Second)
}
