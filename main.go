/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"os"

	"github.com/vipcxj/argonaut/cmd"
)

func main() {
	code := cmd.Execute()
	os.Exit(code)
}
