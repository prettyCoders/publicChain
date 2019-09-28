package main

import (
	"bytes"
	"crypto/elliptic"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

const PeerFile = "peer"

//对等节点
type Peer struct {
	Address string //对等节点的地址（23.65.88.11:3000）
	Type    string //对等节点的类型
	Mining  bool   //对等节点是否开启挖矿
}

//对等节点列表
type Peers struct {
	PeerList []Peer
}

//加载本地的对等节点列表
func LoadPeersFromFile() (*Peers, error) {
	if _, err := os.Stat(PeerFile); os.IsNotExist(err) {
		peers := getSeedPeers()
		peers.SaveToFile()
		return peers, nil
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
	fmt.Println("当前peer数:", len(peers.PeerList))

	return &peers, err
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

func getSeedPeers() *Peers {
	var seedPeers [4]Peer
	seedPeers[0] = Peer{
		Address: "172.31.36.40:8099",
		Type:    "full",
		Mining:  true,
	}
	seedPeers[1] = Peer{
		Address: "172.31.36.29:8099",
		Type:    "full",
		Mining:  true,
	}
	seedPeers[2] = Peer{
		Address: "172.31.36.31:8099",
		Type:    "full",
		Mining:  false,
	}
	seedPeers[3] = Peer{
		Address: "172.31.36.30:8099",
		Type:    "full",
		Mining:  false,
	}
	return &Peers{PeerList: seedPeers[:]}
}
