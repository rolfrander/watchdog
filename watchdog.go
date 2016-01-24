package main

import (
	"fmt"
	"net"
	"os"
)

type Message struct {
	sender *net.UDPAddr
	data []byte
	msgCounter int
}

func (m *Message) String() string {
	return fmt.Sprintf("Message{cnt: %v, sender: %v, data:\"%v\"}",
		m.msgCounter, m.sender, string(m.data))
}

func server(serverAddr *net.UDPAddr, c chan *Message) {
	udp, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting listener: %v\n", err)
		return
	}
	buffer := make([]byte, 1500)
	i := 0
	for {
		cnt, peerAddr, err := udp.ReadFromUDP(buffer)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading from udp port, %v\n", err)
		} else {
			m := &Message{}
			m.sender = peerAddr
			m.data = make([]byte, cnt)
			m.msgCounter = i
			copy(m.data, buffer[0:cnt])
			c <- m
		}
		if cnt == 1 {
			close(c)
			return
		}
		i++
	}
}

func main() {
	fmt.Printf("Start\n")
	serverAddr,err := net.ResolveUDPAddr("udp", "0.0.0.0:4041")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving UDP-address: %v\n", err)
		return
	}

	c := make(chan *Message, 16)
	go server(serverAddr, c)

	for i := range c {
		fmt.Printf("Recieved %v\n", i)
	}
}
