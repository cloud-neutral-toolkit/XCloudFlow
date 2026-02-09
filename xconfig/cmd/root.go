package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "xconfig",
	Short: "Xconfig - 执行与编织任务和架构的现代工具",
	Long:  `Xconfig 是一个现代化的 DevOps CLI 工具，融合任务执行、架构编排、拓扑建模与插件生态。`,
	Run: func(cmd *cobra.Command, args []string) {
		printBanner()
	},
}

// 入口函数
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(
		&DSN,
		"dsn",
		os.Getenv("DATABASE_URL"),
		"PostgreSQL DSN for xcf.* state/memory (defaults to DATABASE_URL)",
	)

	// 注册全局标志（所有子命令可用）
	rootCmd.PersistentFlags().BoolVarP(
		&AggregateOutput,
		"aggregate", "A",
		false,
		"Aggregate output from multiple hosts",
	)
	rootCmd.PersistentFlags().BoolVarP(
		&DiffMode,
		"diff", "D",
		false,
		"when changing (small) files and templates, show the differences in those files",
	)
}

// 启动时打印 ASCII Banner
func printBanner() {
	content, err := os.ReadFile("banner.txt")
	if err == nil {
		fmt.Println(string(content))
	}
}

func addCommandOnce(parent *cobra.Command, child *cobra.Command) {
	for _, existing := range parent.Commands() {
		if existing == child || existing.Name() == child.Name() {
			return
		}
	}
	parent.AddCommand(child)
}
