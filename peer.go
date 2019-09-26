package main

import (
	"bytes"
	"crypto/elliptic"
	"encoding/gob"
	"io/ioutil"
	"log"
	"os"
)

const PeerFile = "peer"

//对等节点
type Peer struct {
	Address string //对等节点的地址（23.65.88.11:3000）
	Type    string //对等节点的类型
	mining  bool   //对等节点是否开启挖矿
}

//对等节点列表
type Peers struct {
	peerList []Peer
}

//加载本地的对等节点列表
func (p *Peers) LoadFromFile() error {
	if _, err := os.Stat(PeerFile); os.IsNotExist(err) {
		return err
	}

	fileContent, err := ioutil.ReadFile(PeerFile)
	if err != nil {
		log.Panic(err)
	}

	var peers Peers
	gob.Register(elliptic.P256())
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&peers)
	if err != nil {
		log.Panic(err)
	}

	p.peerList = peers.peerList

	return nil
}

//保存最新的对等节点列表
func (p Peers) SaveToFile() {
	var content bytes.Buffer
	gob.Register(elliptic.P256())

	encoder := gob.NewEncoder(&content)
	err := encoder.Encode(p)
	if err != nil {
		log.Panic(err)
	}

	err = ioutil.WriteFile(PeerFile, content.Bytes(), 0644)
	if err != nil {
		log.Panic(err)
	}
}

func GetSeedPeers() *Peers {
	var seedPeers [4]Peer
	seedPeers[0] = Peer{
		Address: "172.31.36.40:8099",
		Type:    "full",
		mining:  true,
	}
	seedPeers[1] = Peer{
		Address: "172.31.36.29:8099",
		Type:    "full",
		mining:  true,
	}
	seedPeers[2] = Peer{
		Address: "172.31.36.31:8099",
		Type:    "full",
		mining:  false,
	}
	seedPeers[3] = Peer{
		Address: "172.31.36.30:8099",
		Type:    "full",
		mining:  false,
	}
	return &Peers{peerList: seedPeers[:]}
}
