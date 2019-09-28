package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math"
	"math/big"
)

/**
区块链的一个关键点就是，一个人必须经过一系列困难的工作，才能将数据放入到区块链中
。正是由于这种困难的工作，才保证了区块链的安全和一致。此外，完成这个工作的人，也会获得相应奖励（这也就是通过挖矿获得币）。
这个机制与生活现象非常类似：一个人必须通过努力工作，才能够获得回报或者奖励，用以支撑他们的生活。
在区块链中，是通过网络中的参与者（矿工）不断的工作来支撑起了整个网络。
矿工不断地向区块链中加入新块，然后获得相应的奖励。
在这种机制的作用下，新生成的区块能够被安全地加入到区块链中，它维护了整个区块链数据库的稳定性。
值得注意的是，完成了这个工作的人必须要证明这一点，即他必须要证明他的确完成了这些工作。
这个 “努力工作并进行证明” 的机制，就叫做工作量证明（proof-of-work）。
要想完成工作非常地不容易，因为这需要大量的计算能力：即便是高性能计算机，也无法在短时间内快速完成。
另外，这个工作的困难度会随着时间不断增长，以保持每 10 分钟出 1 个新块的速度(Bitcoin是这样设计的)。
在比特币中，这个工作就是找到一个块的哈希，同时这个哈希满足了一些必要条件。
这个哈希，也就充当了证明的角色。因此，寻求证明（寻找有效哈希），就是矿工实际要做的事情
*/

var (
	maxNonce = math.MaxInt64 //计数器最大值,对pow的次数进行限制
)

/**
在比特币中，当一个块被挖出来以后，“target bits” 代表了区块头里存储的难度，也就是开头有多少个 0。
这里的 24 指的是算出来的哈希前 24 位必须是 0，如果用 16 进制表示，就是前 6 位必须是 0，这一点从最后的输出可以看出来
*/
const targetBits = 16

type ProofOfWork struct {
	block  *Block   //区块
	target *big.Int //目标，这里使用了一个 大整数，我们会将哈希与目标进行比较：先把哈希转换成一个大整数，然后检测它是否小于目标。
}

// 构建并返回新的pow对象
func NewProofOfWork(b *Block) *ProofOfWork {
	//将 big.Int 初始化为 1，16 进制形式为:0x10000000000000000000000000000000000000000000000000000000000
	// 然后左移 256 - targetBits 位,16 进制形式为:0000010000000000000000000000000000000000000000000000000000000000
	// 256 是一个 SHA-256 哈希的位数，我们将要使用的是 SHA-256 哈希算法。
	//比如对"I like donuts"做Hash，结果为0fac49161af82ed938add1d8725835cc123a1a87b1b196488360e58d4bfb51e3
	//显然比当前的target要大，所以不满足条件
	//但是对"I like donutsca07ca"做Hash，结果为0000008b0f41ec78bab747864db66bcb9fb89920ee75f43fdaaeb5544f7f76ca
	//显然比当前的target要小，所以满足条件，ca07ca 是nonce的 16 进制值，十进制的话是 13240266.也就是计算了13240266次Hash
	target := big.NewInt(1)
	target.Lsh(target, uint(256-targetBits))

	pow := &ProofOfWork{b, target}

	return pow
}

//准备数据,只需要将 target ，nonce 与 Block 进行合并
func (pow *ProofOfWork) prepareData(nonce int) []byte {
	data := bytes.Join(
		[][]byte{
			pow.block.PrevBlockHash,
			pow.block.HashTransactions(),
			IntToHex(pow.block.Timestamp),
			IntToHex(int64(targetBits)),
			IntToHex(int64(nonce)),
		},
		[]byte{},
	)

	return data
}

// 执行pow
func (pow *ProofOfWork) Run() (int, []byte) {
	var hashInt big.Int //hash 的整形表示
	var hash [32]byte
	nonce := 0 //计数器初始值为0

	fmt.Printf("Mining a new block")
	for nonce < maxNonce {
		data := pow.prepareData(nonce) //准备数据

		hash = sha256.Sum256(data) //用 SHA-256 对数据进行哈希
		fmt.Printf("\r%x", hash)
		hashInt.SetBytes(hash[:]) //将哈希转换成一个大整数

		if hashInt.Cmp(pow.target) == -1 { //将这个大整数与目标进行比较
			break
		} else {
			nonce++
		}
	}
	fmt.Print("\n\n")

	return nonce, hash[:]
}

// 验证区块的pow是否合法
func (pow *ProofOfWork) Validate() bool {
	var hashInt big.Int
	//取出当前区块pow之后得出的nonce值，准备再次pow的数据
	//这个数据其实就是当前区块pow成功的最终数据
	data := pow.prepareData(pow.block.Nonce)
	hash := sha256.Sum256(data) //对数据进行Hash计算
	hashInt.SetBytes(hash[:])

	isValid := hashInt.Cmp(pow.target) == -1 //比较

	return isValid
}
