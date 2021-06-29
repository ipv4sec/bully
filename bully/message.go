package bully

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

const msgBlockSize = 128

type MessageType uint8
const (
	ElectionMessage MessageType = iota
	OKMessage
	CoordinatorMessage
)

type Message struct {
	Kind  MessageType
	Rank  uint64
	IP   *net.IP
	Tag   string
}


func NewMessageFromString(data string) *Message {
	tokens := strings.Split(data, "|")
	kind, _ := strconv.ParseUint(tokens[0], 10, 8)
	ip := net.ParseIP(tokens[2])
	rank, _ := strconv.ParseUint(tokens[1], 10, 64)
	return &Message{Tag: tokens[3], Kind: MessageType(kind), IP: &ip, Rank: rank}
}

func (m *Message) Pack() string {
	ipString := net.IP.String(*m.IP)
	transmitData := fmt.Sprintf("%d|%d|%s|%s|", m.Kind, m.Rank, ipString, m.Tag)
	return transmitData + strings.Repeat("#", msgBlockSize-len(transmitData))
}
