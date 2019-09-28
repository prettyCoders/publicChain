package main

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"sync"
)

const protocol = "tcp"
const listenPort = 8099

//commandLength 表示命令名长度。
// 节点之间交互的消息，在底层就是字节序列。前 20 个字节指定了命令名（比如 version），后面的字节会包含 gob 编码的消息结构
const commandLength = 20

var lock sync.Mutex //互斥锁

var blocksInTransit = [][]byte{}           //保存已下载的块
var mempool = make(map[string]Transaction) //交易内存池
var Mining bool                            //节点是否开启挖矿
var node *Node                             //当前节点

//比特币使用 Inv 来向其他节点展示当前节点有什么块和交易。
// 再次提醒，它没有包含完整的区块链和交易，仅仅是哈希而已。Type 字段表明了这是块还是交易
type Inv struct {
	NodeInfo Node
	Type     string
	Items    [][]byte
}

//用于某个块或交易的请求，它可以仅包含一个块或交易的 Hash
type Data struct {
	NodeInfo Node
	Type     string
	Hash     []byte
}

type BlockData struct {
	NodeInfo Node
	Block    []byte
}

type TxData struct {
	NodeInfo    Node
	Transaction []byte
}

// StartServer starts a node
//节点启动之后需要做的工作
//1、初始化节点信息
//2、加载本地保存的peer
//3、然后发送版本信息
func StartServer() {
	bc := NewBlockchain()
	//初始化节点信息
	initNode(Mining, bc)

	if Mining {
		go mining(bc)
	}

	//加载本地保存的peer
	var err error
	peers, err := LoadPeersFromFile()
	//开启服务

	ln, err := net.Listen(protocol, fmt.Sprintf("0.0.0.0:%d", listenPort))
	if err != nil {
		log.Panic(err)
	}
	defer ln.Close()

	//发送节点信息到其他节点，以便加入网络
	fmt.Printf("当前机器的内网IP为：%s\n", fmt.Sprintf("%s:%d", GetInternalIp(), listenPort))
	for _, peer := range peers.PeerList {
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

//更新节点信息
func UpdateNode(mining bool, bc *Blockchain) {
	node = NewNode("full", mining, bc)
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

//发送当前节点信息
//peerAddress 目标节点地址
func sendNodeMessage(peerAddress string, bc *Blockchain) error {
	payload := gobEncode(node)
	request := append(commandToBytes("node"), payload...)
	return sendData(peerAddress, request)
}

/**
处理其他节点发送过来的节点信息
1、验证peer版本是否匹配，如果不匹配，直接忽略这次请求
2、如果是新peer,保存peer信息
3、如果peer高度低于本节点，则发送它缺少的区块的Hash列表给他
4、如果peer高度高于本节点，则请求查看本节点缺少的区块的Hash列表
*/

func handleNodeMessage(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var peerNode Node

	//提取消息内容并解码
	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&peerNode)
	if err != nil {
		log.Panic(err)
	}

	myVersion := node.Version
	foreignerVersion := peerNode.Version
	if myVersion == foreignerVersion {
		myBestHeight := bc.GetBestHeight()
		foreignerBestHeight := peerNode.BestBlockHeight
		fmt.Printf("收到节点的NodeMessage请求，当前节点BestBlockHeight:%d，对方BestBlockHeight:%d\n", myBestHeight, foreignerBestHeight)
		peers, _ := LoadPeersFromFile()
		//新peer
		if !peerNode.isOld() {
			peers.PeerList = append(peers.PeerList, Peer{
				Address: peerNode.Address,
				Type:    peerNode.Type,
				mining:  peerNode.Mining,
			})
			peers.SaveToFile()
		}

		//如果peer高度低于本节点，则发送它缺少的区块的Hash列表给他
		if myBestHeight > foreignerBestHeight {
			blockHashes := bc.GetBlockHashes(peerNode.BestBlockHeight)
			sendInv(peerNode.Address, "higherBlockHashes", blockHashes)
			//如果peer高度高于本节点，则请求查看本节点缺少的区块的Hash列表
		} else if myBestHeight < foreignerBestHeight {
			getHigherBlockHashes(peerNode)
		}
	}
}

//获取peer比本节点更高的区块Hash列表
func getHigherBlockHashes(peerNode Node) {
	payload := gobEncode(node)
	request := append(commandToBytes("getHigherBlockHashes"), payload...)
	sendData(peerNode.Address, request)
}

//处理来自peer的“给我更高的区块Hash列表”的请求
func handleGetHigherBlockHashes(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var peerNode Node

	//提取消息内容并解码
	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&peerNode)
	if err != nil {
		log.Panic(err)
	}

	blockHashes := bc.GetBlockHashes(peerNode.BestBlockHeight)
	sendInv(peerNode.Address, "higherBlockHashes", blockHashes)
}

//发送inv，也就是向其他节点展示当前节点有什么块和交易
func sendInv(address, kind string, items [][]byte) {
	inventory := Inv{*node, kind, items}
	payload := gobEncode(inventory)
	request := append(commandToBytes("Inv"), payload...)
	sendData(address, request)
}

//处理其他节点发送过来的块Hash或者交易Hash
func handleInv(request []byte, blockchain *Blockchain) {
	var buff bytes.Buffer
	var payload Inv

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("Recevied inventory with %d %s\n", len(payload.Items), payload.Type)

	//如果收到块哈希，我们想要将它们保存在 blocksInTransit 变量来跟踪已下载的块。这能够让我们从不同的节点下载块。
	// 在将块置于传送状态时，我们给 Inv 消息的发送者发送 getdata 命令并更新 blocksInTransit。
	// 在一个真实的 P2P 网络中，我们会想要从不同节点来传送块。
	if payload.Type == "higherBlockHashes" {
		//保存已下载的块
		blocksInTransit = payload.Items

		blockHash := payload.Items[0]
		//给 Inv 消息的发送者发送 getdata 命令并更新 blocksInTransit
		sendGetData(payload.NodeInfo.Address, "block", blockHash)

		newInTransit := [][]byte{}
		for _, b := range blocksInTransit {
			if bytes.Compare(b, blockHash) != 0 {
				newInTransit = append(newInTransit, b)
			}
		}
		blocksInTransit = newInTransit
	}

	//在我们的实现中，我们永远也不会发送有多重哈希的 Inv。这就是为什么当 payload.Type == "tx" 时，只会拿到第一个哈希。
	//然后我们检查是否在内存池中已经有了这个哈希，如果没有，发送 getdata 消息
	if payload.Type == "tx" {
		txID := payload.Items[0]

		if mempool[hex.EncodeToString(txID)].ID == nil {
			sendGetData(payload.NodeInfo.Address, "tx", txID)
		}
	}
}

//发送获取数据的请求,目前用于本节点向peer请求区块数据或交易数据，参数为Hash
func sendGetData(address, kind string, id []byte) {
	payload := gobEncode(Data{*node, kind, id})
	request := append(commandToBytes("getData"), payload...)
	sendData(address, request)
}

//这个处理器比较地直观：如果它们请求一个块，则返回块；如果它们请求一笔交易，则返回交易。
//TODO:注意，我们并不检查实际上是否已经有了这个块或交易。这是一个缺陷
func handleGetData(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var data Data

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&data)
	if err != nil {
		log.Panic(err)
	}

	if data.Type == "block" {
		block, err := bc.GetBlock([]byte(data.Hash))
		if err != nil {
			return
		}

		sendBlock(data.NodeInfo.Address, &block)
	}

	if data.Type == "tx" {
		txID := hex.EncodeToString(data.Hash)
		tx := mempool[txID]

		sendTx(data.NodeInfo.Address, &tx)
		// delete(mempool, txID)
	}
}

//发送区块数据
func sendBlock(addr string, b *Block) {
	data := BlockData{*node, b.Serialize()}
	payload := gobEncode(data)
	request := append(commandToBytes("blockData"), payload...)

	sendData(addr, request)
}

//当接收到一个新块时，我们把它放到区块链里面。如果还有更多的区块需要下载，我们继续从上一个下载的块的那个节点继续请求。
// 当最后把所有块都下载完后，对 UTXO 集进行重新索引。
//TODO：并非无条件信任，我们应该在将每个块加入到区块链之前对它们进行验证。
//TODO: 并非运行 UTXOSet.Reindex()， 而是应该使用 UTXOSet.Update(block)，因为如果区块链很大，
// 它将需要很多时间来对整个 UTXO 集重新索引
func handleBlock(request []byte, bc *Blockchain) {
	lock.Lock()
	defer lock.Unlock()
	var buff bytes.Buffer
	var blockData BlockData

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&blockData)
	if err != nil {
		log.Panic(err)
	}

	blockBytes := blockData.Block
	block := DeserializeBlock(blockBytes)
	//验证区块有效性
	//判断区块是否已经存在
	_, err = bc.GetBlock(block.Hash)
	if err == nil {
		fmt.Println("区块已经存在！")
		return
	}

	fmt.Println("Recevied a new block!")
	bc.AddBlock(block)

	fmt.Printf("Added block %x\n", block.Hash)

	if len(blocksInTransit) > 0 {
		blockHash := blocksInTransit[0]
		sendGetData(blockData.NodeInfo.Address, "block", blockHash)
		blocksInTransit = blocksInTransit[1:]
	} else {
		UTXOSet := UTXOSet{bc}
		UTXOSet.Reindex()
	}
}

//发送交易数据
func sendTx(addr string, tnx *Transaction) {
	data := TxData{*node, tnx.Serialize()}
	payload := gobEncode(data)
	request := append(commandToBytes("txData"), payload...)
	sendData(addr, request)
}

//处理其他节点发送过来的交易数据
func handleTx(request []byte, bc *Blockchain) {
	var buff bytes.Buffer
	var txData TxData

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&txData)
	if err != nil {
		log.Panic(err)
	}

	txBytes := txData.Transaction
	tx := DeserializeTransaction(txBytes)
	//首先要做的事情是将新交易放到内存池中
	//TODO:在将交易放到内存池之前，必要对其进行验证
	mempool[hex.EncodeToString(tx.ID)] = tx

	///**
	//收到新的交易
	//判断交易的有效性，放到交易池即可
	// */
	////检查当前节点是否是中心节点。在我们的实现中，中心节点并不会挖矿。它只会将新的交易推送给网络中的其他节点
	//if nodeAddress == knownNodes[0] {
	//	for _, node := range knownNodes {
	//		if node != nodeAddress && node != txData.AddFrom {
	//			sendInv(node, "tx", [][]byte{tx.ID})
	//		}
	//	}
	//} else {
	//	//如果当前节点是旷工节点并且交易池交易数量大于等于2
	//	if len(mempool) >= 2 && len(miningAddress) > 0 {
	//	MineTransactions:
	//		var txs []*Transaction
	//		//首先，内存池中所有交易都是通过验证的。无效的交易会被忽略，如果没有有效交易，则挖矿中断
	//		for id := range mempool {
	//			tx := mempool[id]
	//			if bc.VerifyTransaction(&tx) {
	//				txs = append(txs, &tx)
	//			}
	//		}
	//
	//		if len(txs) == 0 {
	//			fmt.Println("All transactions are invalid! Waiting for new ones...")
	//			return
	//		}
	//
	//		//验证后的交易被放到一个块里，同时还有附带奖励的 coinbase 交易。当块被挖出来以后，UTXO 集会被重新索引。
	//		//TODO: 提醒，应该使用 UTXOSet.Update 而不是 UTXOSet.Reindex.
	//		cbTx := NewCoinbaseTX(miningAddress, "")
	//		txs = append(txs, cbTx)
	//
	//		newBlock := bc.MineBlock(txs)
	//		UTXOSet := UTXOSet{bc}
	//		UTXOSet.Reindex()
	//
	//		fmt.Println("New block is mined!")
	//
	//		//当一笔交易被挖出来以后，就会被从内存池中移除。
	//		// 当前节点所连接到的所有其他节点，接收带有新块哈希的 inv 消息。在处理完消息后，它们可以对块进行请求
	//		for _, tx := range txs {
	//			txID := hex.EncodeToString(tx.ID)
	//			delete(mempool, txID)
	//		}
	//
	//		for _, node := range knownNodes {
	//			if node != nodeAddress {
	//				sendInv(node, "block", [][]byte{newBlock.Hash})
	//			}
	//		}
	//
	//		if len(mempool) > 0 {
	//			goto MineTransactions
	//		}
	//	}
	//}
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
	//处理其他节点发送过来的node信息
	case "node":
		handleNodeMessage(request, bc)
		//处理其他节点发送过来比本节点更高的区块Hash列表
	case "getHigherBlockHashes":
		handleGetHigherBlockHashes(request, bc)
		//处理其他节点发送过来的块Hash或者交易Hash
	case "Inv":
		handleInv(request, bc)
		//处理其他节点发送过来的"根据Hash获取区块或者交易"的请求
	case "getData":
		handleGetData(request, bc)
	case "blockData":
		handleBlock(request, bc)
	case "txData":
		handleTx(request, bc)
	default:
		fmt.Println("Unknown command!")
	}

	conn.Close()
}

func mining(bc *Blockchain) {
	fmt.Println("开始挖矿")
	for Mining {
		//如果节点开启挖矿，则在挖矿的同时，不停的取交易池的数据打包进区块
		//挖矿成功后广播给peer
		wallets, err := NewWallets()
		if err != nil {
			log.Panic(err)
		}

		UTXOSet := UTXOSet{bc}
		cbTx := NewCoinbaseTX(wallets.CreateWallet(), "")
		txs := []*Transaction{}
		txs = append(txs, cbTx)
		for hash, transaction := range mempool {
			txs = append(txs, &transaction)
			delete(mempool, hash)
		}
		newBlock := bc.MineBlock(txs)
		UTXOSet.Update(newBlock)
		go shareMyBooty(bc)
	}
}

//挖矿成功，通知其他节点来同步数据
func shareMyBooty(bc *Blockchain) {
	peers, _ := LoadPeersFromFile()
	for _, peer := range peers.PeerList {
		peerAddress := peer.Address
		//自己的外网IP
		if peerAddress == fmt.Sprintf("%s:%d", GetInternalIp(), listenPort) {
			continue
		}
		sendNodeMessage(peer.Address, bc)
	}
}
