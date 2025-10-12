package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"voiceflow/pkg/maimemo"
)

func main() {
	// 定义命令行参数
	token := flag.String("token", "", "墨墨 API Token（从 APP 获取：我的 > 更多设置 > 实验功能 > 开放API）")
	flag.Parse()

	// 检查 token
	if *token == "" {
		fmt.Println("❌ 错误：请提供墨墨 API Token")
		fmt.Println("\n使用方法：")
		fmt.Println("  go run cmd/list-notepads/main.go -token=YOUR_TOKEN")
		fmt.Println("\nToken 获取方式：")
		fmt.Println("  打开墨墨 APP → 我的 → 更多设置 → 实验功能 → 开放API")
		os.Exit(1)
	}

	// 创建客户端
	client := maimemo.NewClient(*token)

	// 获取云词本列表
	fmt.Println("🔍 正在获取云词本列表...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	notepads, err := client.ListNotepads(ctx)
	if err != nil {
		fmt.Printf("❌ 获取失败: %v\n", err)
		os.Exit(1)
	}

	// 显示结果
	if len(notepads) == 0 {
		fmt.Println("\n📭 您还没有创建任何云词本")
		fmt.Println("\n💡 提示：")
		fmt.Println("  1. 打开墨墨 APP")
		fmt.Println("  2. 进入「我的」→「云词本」")
		fmt.Println("  3. 创建一个新的云词本")
		return
	}

	fmt.Printf("\n✅ 找到 %d 个云词本：\n\n", len(notepads))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for i, notepad := range notepads {
		fmt.Printf("📚 词本 %d\n", i+1)
		fmt.Printf("   名称: %s\n", notepad.Title)
		fmt.Printf("   ID:   %s  👈 复制这个ID用于同步\n", notepad.ID)
		fmt.Printf("   简介: %s\n", notepad.Brief)
		if len(notepad.Tags) > 0 {
			fmt.Printf("   标签: %v\n", notepad.Tags)
		}
		if i < len(notepads)-1 {
			fmt.Println("   ────────────────────────────────────────")
		}
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("\n💡 使用方法：")
	fmt.Println("  复制上面的词本ID，在前端同步时粘贴即可")
}
