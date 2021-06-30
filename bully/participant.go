package bully

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"
)

const UsurpInterval = 4000 * time.Millisecond
const PublishInterval = 250 * time.Millisecond
const ElectionTimeout = 2000 * time.Millisecond

type Participant struct {
	sync.Mutex
	AfterFunc *time.Timer
	PublishAt time.Time // 广而告之
	UsurpAt   time.Time // 准备篡位
	Happening chan Node
	Transport Transport

	IP   *net.IP
	Rank uint64
	Tag  string

	Master Node
	Status Status
	Callback func(string, string, uint64)
}

type Status int
const (
	SlaveStatus Status = iota
	MasterStatus
)

type Node struct {
	Rank uint64
	Addr string
}

func NewParticipant(UDPAddrString, interfaceName string, rank uint64, tag string, callback func(string, string, uint64)) (*Participant, error) {
	UDPAddr, err := net.ResolveUDPAddr("udp", UDPAddrString)
	if err != nil {
		return nil, err
	}
	ifi, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, err
	}

	transport, err := NewMulticastTransport(&UDPAddr.IP, ifi, UDPAddr.Port)
	if err != nil {
		return nil, err
	}
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil, err
	}
	var sourceIP *net.IP
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				sourceIP = &ipnet.IP
				break
			}
		}
	}
	if sourceIP == nil {
		return nil, errors.New("missing source IP")
	}
	p := &Participant{
		IP:        sourceIP,
		Rank:      rank,
		Tag:       tag,
		Transport: transport,
		Happening: make(chan Node, 1024),
		UsurpAt: time.Now().Add(UsurpInterval + ElectionTimeout),
		Status: SlaveStatus,
		Callback: callback,
	}
	return p, nil
}

func (p *Participant) Run(done chan interface{}) {
	go func() {
		for {
			msg, err := p.Transport.Read()
			if err != nil {
				log.Println(err)
				continue
			}
			log.Printf("r 接收 %+v", msg)
			if msg != nil {
				if msg.Tag != p.Tag {
					continue
				}
				p.handleMessage(msg)
			}
		}
	}()

	go func() {
		for {
			if time.Now().After(p.UsurpAt) && p.Status == SlaveStatus {
				log.Println("* 皇帝驾崩, 我要篡位")
				p.StartElection()
			}
			time.Sleep(UsurpInterval)
		}
	}()

	go func() {
		for {
			p.Lock()
			p.PublishAt = p.PublishAt.Add(PublishInterval)
			p.Unlock()
			if time.Now().After(p.PublishAt) && p.Status == MasterStatus {
				log.Println("* 广而告之, 我是皇帝")
				p.sendMessage(CoordinatorMessage)
			}
			time.Sleep(PublishInterval)
		}
	}()

	go func() {
		for {
			v := <-p.Happening
			log.Println("v", v)
			p.Lock()
			p.UsurpAt = p.UsurpAt.Add(UsurpInterval)
			p.Unlock()
			if v.Addr == p.IP.String() && v.Rank == p.Rank {
				p.Status = MasterStatus
			} else {
				p.Status = SlaveStatus
			}
			p.Master = v
			p.Callback(p.IP.String(), v.Addr, v.Rank)
		}
	}()

	p.StartElection()
	<-done
	err := p.Transport.Close()
	if err != nil {
		log.Println(err)
	}
}

func (p *Participant) handleMessage(rowMessage *Message) {
	switch rowMessage.Kind {
	case ElectionMessage:
		p.handleElectionMessage(rowMessage)
	case OKMessage:
		p.handleOKMessage(rowMessage)
	case CoordinatorMessage:
		p.handleCoordinatorMessage(rowMessage)
	}
}

func (p *Participant) handleElectionMessage(rowMessage *Message) {
	if p.Rank < rowMessage.Rank {
		p.Status = SlaveStatus
		p.stopElection()
	}
}
func (p *Participant) handleOKMessage(rowMessage *Message) {
	if p.Rank < rowMessage.Rank {
		p.Status = SlaveStatus
		p.stopElection()
	}
}

func (p *Participant) handleCoordinatorMessage(rowMessage *Message) {
	if p.Rank > rowMessage.Rank {
		log.Print("* 无耻之徒, 抢我江山")
		p.StartElection()
	} else {
		p.stopElection()
		p.Lock()
		p.UsurpAt = p.UsurpAt.Add(PublishInterval)
		p.Unlock()
		if p.Master.Rank < rowMessage.Rank {
			p.Happening <- Node {
				Rank: rowMessage.Rank,
				Addr: rowMessage.IP.String(),
			}
		}
	}
}

func (p *Participant) StartElection() {
	p.stopElection()
	p.sendMessage(ElectionMessage)
	p.AfterFunc = time.AfterFunc(ElectionTimeout, func() {
		log.Println("* 受命于天, 既寿永昌")
		p.Happening <- Node{
			Rank: p.Rank,
			Addr: p.IP.String(),
		}
	})
}

func (p *Participant) stopElection() {
	if p.AfterFunc != nil {
		if !p.AfterFunc.Stop() {
			go func() {
				select {
				case <-p.AfterFunc.C:
				case <-time.After(time.Second):
				}
			}()
		}
	}
}

func (p *Participant) sendMessage(kind MessageType) {
	msg := &Message{Kind: kind, Rank: p.Rank, Tag: p.Tag, IP: p.IP}
	log.Printf("s 发送 %+v", msg)
	err := p.Transport.Write(msg)
	if err != nil {
		log.Println(err)
	}
}