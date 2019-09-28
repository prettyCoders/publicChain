package main

import (
	"fmt"
)

func (cli *CLI) startNode(mine bool) {
	fmt.Printf("Starting node\n")
	Mining = mine
	StartServer()
}
