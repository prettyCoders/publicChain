package main

/**
节点
节点暂时分为全节点，轻节点
全节点可以选择开启或关闭挖矿功能
1、当全节点关闭挖矿功能的时候，其他节点不会向它推送交易，只会推送新的区块
也就是说关闭挖矿的全节点只负责校验和同步区块链
2、当全节点开启挖矿功能的时候，其他节点会向它推送交易和区块，此时节点需要校验交易和区块的合法性
然后将交易放到交易池，每一笔都需要结合现有区块链的UTXO和交易池的交易加起来进行校验。
*/

const NodeVersion = "0.0.1"

type Node struct {
	Version         string //版本号
	Type            string //类型
	Mining          bool   //是否开启挖矿
	BestBlockHeight int    //最新区块高度
}

//创建新的Node对象
func NewNode(nodeType string, mining bool, blockchain *Blockchain) *Node {
	return &Node{
		Version:         NodeVersion,
		Type:            nodeType,
		Mining:          mining,
		BestBlockHeight: blockchain.GetBestHeight(),
	}
}
