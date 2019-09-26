package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
)

const protocol = "tcp"
const listenPort = 8099

//commandLength 表示命令名长度。
// 节点之间交互的消息，在底层就是字节序列。前 12 个字节指定了命令名（比如 version），后面的字节会包含 gob 编码的消息结构
const commandLength = 12

var node *Node           //当前节点
var miningAddress string //接收挖矿奖励的地址

// StartServer starts a node
//节点启动之后需要做的工作
//1、初始化节点信息
//2、加载本地保存的peer
//3、然后发送版本信息
func StartServer(minerAddress string) {
	//初始化节点信息
	mining := false
	if len(minerAddress) > 0 {
		miningAddress = minerAddress
		mining = true
	}
	bc := NewBlockchain()
	initNode(mining, bc)

	//加载本地保存的peer
	var peers = &Peers{nil}
	err := peers.LoadFromFile()
	if err != nil {
		peers = GetSeedPeers()
	}

	//开启服务

	ln, err := net.Listen(protocol, fmt.Sprintf("0.0.0.0:%d", listenPort))
	if err != nil {
		log.Panic(err)
	}
	defer ln.Close()

	//发送节点信息到其他节点，以便加入网络
	fmt.Printf("当前机器的内网IP为：%s\n", fmt.Sprintf("%s:%d", GetInternalIp(), listenPort))
	for _, peer := range peers.peerList {
		peerAddress := peer.Address
		//自己的外网IP
		if peerAddress == fmt.Sprintf("%s:%d", GetInternalIp(), listenPort) {
			continue
		}
		fmt.Printf("正在连接至节点%s", peerAddress)
		err := sendNodeMessage(peerAddress, bc)
		if err != nil {
			fmt.Println("\t失败")
			continue
		}
		fmt.Println("\t成功")
	}

	//开启监听
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Panic(err)
		}
		go handleConnection(conn, bc)
	}

}

//初始化节点
func initNode(mining bool, bc *Blockchain) {
	node = NewNode("full", mining, bc)
}

//发送当前节点信息
//peerAddress 目标节点地址
func sendNodeMessage(peerAddress string, bc *Blockchain) error {
	payload := gobEncode(node)
	request := append(commandToBytes("node"), payload...)
	return sendData(peerAddress, request)
}

//发送数据
func sendData(addr string, data []byte) error {
	conn, err := net.Dial(protocol, addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = io.Copy(conn, bytes.NewReader(data))
	return err
}

// gob 编码数据
func gobEncode(data interface{}) []byte {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

//命令转字节数组
//创建一个 12 字节的缓冲区，并用命令名进行填充，将剩下的字节置为空。
func commandToBytes(command string) []byte {
	var bytes [commandLength]byte

	for i, c := range command {
		bytes[i] = byte(c)
	}

	return bytes[:]
}

//字节数组转命令
func bytesToCommand(bytes []byte) string {
	var command []byte

	for _, b := range bytes {
		if b != 0x0 {
			command = append(command, b)
		}
	}
	return fmt.Sprintf("%s", command)
}

//处理其他节点的请求
func handleConnection(conn net.Conn, bc *Blockchain) {
	fmt.Println("远程地址：", conn.RemoteAddr())
	request, err := ioutil.ReadAll(conn)
	if err != nil {
		log.Panic(err)
	}
	//提取出命令名
	command := bytesToCommand(request[:commandLength])
	fmt.Printf("Received %s command\n", command)

	//根据命令名选择相应的处理器
	switch command {
	case "node":
		handleNodeMessage(request, bc)
	default:
		fmt.Println("Unknown command!")
	}

	conn.Close()
}

//处理其他节点发送过来的节点信息
func handleNodeMessage(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var payload Node

	//提取消息内容并解码
	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	myBestHeight := bc.GetBestHeight()
	foreignerBestHeight := payload.BestBlockHeight
	fmt.Printf("收到节点的NodeMessage请求，当前节点BestBlockHeight:%d，对方BestBlockHeight:%d\n", myBestHeight, foreignerBestHeight)
}
