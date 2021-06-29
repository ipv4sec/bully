package bully

import (
	"errors"
	"log"
	"net"
)

type Transport interface {
	Read() (*Message, error)
	Write(*Message) error
	Close() error
}

type MulticastTransport struct {
	readConn  *net.UDPConn
	writeConn *net.UDPConn
	buffer    []byte
}

func NewMulticastTransport(addr *net.IP, ifi *net.Interface, port int) (*MulticastTransport, error) {
	if !addr.IsMulticast() {
		return nil, errors.New("cant Multicast")
	}
	UDPAddr := &net.UDPAddr{IP: *addr, Port: port}
	readConn, err := net.ListenMulticastUDP("udp", ifi, UDPAddr)
	if err != nil {
		return nil, err
	}
	writeConn, err := net.DialUDP("udp", nil, UDPAddr)
	if err != nil {
		return nil, err
	}
	return &MulticastTransport{readConn: readConn, writeConn: writeConn, buffer: []byte{}}, nil
}

func (t *MulticastTransport) Read() (*Message, error) {
	readBuffer := make([]byte, 1500)
	var msg *Message
	var err error
Loop:
	for {
		num, _, e := t.readConn.ReadFrom(readBuffer)
		if err != nil {
			log.Println(err)
			err = e
		}

		t.buffer = append(t.buffer, readBuffer[:num]...)
		if len(t.buffer) >= msgBlockSize {
			data := t.buffer[:msgBlockSize]
			msg = NewMessageFromString(string(data))
			t.buffer = t.buffer[msgBlockSize:]
			break Loop
		}
	}

	return msg, err
}

func (t *MulticastTransport) Write(m *Message) error {
	_, err := t.writeConn.Write([]byte(m.Pack()))
	return err
}

func (t *MulticastTransport) Close() error {
	err := t.readConn.Close()
	if err != nil {
		return err
	}
	return t.writeConn.Close()
}