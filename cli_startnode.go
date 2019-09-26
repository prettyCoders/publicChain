package main

import (
	"fmt"
	"log"
)

func (cli *CLI) startNode(minerAddress string) {
	fmt.Printf("Starting node\n")

	//如果指定了挖矿收益地址，那么节点默认为旷工节点
	if len(minerAddress) > 0 {
		if ValidateAddress(minerAddress) {
			fmt.Println("Mining is on. Address to receive rewards: ", minerAddress)
		} else {
			log.Panic("Wrong miner Address!")
		}
	}
	StartServer(minerAddress)
}
