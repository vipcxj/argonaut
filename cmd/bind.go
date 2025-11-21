/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/vipcxj/argonaut/internal/bind"

	"github.com/spf13/cobra"
)

// bindCmd represents the bind command
var bindCmd = &cobra.Command{
	Use:                "bind",
	Short:              bind.ShortDesc,
	Long:               bind.LongDesc,
	DisableFlagParsing: true,
	Run:                bind.Run,
}

func init() {
	rootCmd.AddCommand(bindCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// bindCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// bindCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
