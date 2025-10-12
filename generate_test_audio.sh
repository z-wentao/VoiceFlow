#!/bin/bash

# VoiceFlow 测试音频生成脚本

echo "🎵 VoiceFlow 测试音频生成工具"
echo "================================"
echo ""

# 检查 FFmpeg 是否安装
if ! command -v ffmpeg &> /dev/null; then
    echo "❌ 错误: 未找到 FFmpeg"
    echo "请先安装 FFmpeg:"
    echo "  macOS:   brew install ffmpeg"
    echo "  Ubuntu:  sudo apt install ffmpeg"
    exit 1
fi

echo "✓ FFmpeg 已安装"
echo ""

# 创建测试目录
TEST_DIR="test_audio"
mkdir -p "$TEST_DIR"
echo "✓ 测试目录: $TEST_DIR"
echo ""

# 生成不同时长的测试音频
echo "📊 开始生成测试音频..."
echo ""

# 1. 8分钟（不会切片）
echo "1️⃣  生成 8分钟音频 (test_8min.mp3) - 无需切片测试"
ffmpeg -f lavfi -i "sine=frequency=440:duration=480" \
  -ar 44100 -b:a 128k -y "$TEST_DIR/test_8min.mp3" \
  -loglevel error

FILE_SIZE=$(du -h "$TEST_DIR/test_8min.mp3" | cut -f1)
echo "   ✓ 完成: $FILE_SIZE"
echo ""

# 2. 15分钟（切2片）
echo "2️⃣  生成 15分钟音频 (test_15min.mp3) - 将切成 2 个片段"
ffmpeg -f lavfi -i "sine=frequency=523:duration=900" \
  -ar 44100 -b:a 128k -y "$TEST_DIR/test_15min.mp3" \
  -loglevel error

FILE_SIZE=$(du -h "$TEST_DIR/test_15min.mp3" | cut -f1)
echo "   ✓ 完成: $FILE_SIZE"
echo ""

# 3. 30分钟（切3片）
echo "3️⃣  生成 30分钟音频 (test_30min.mp3) - 将切成 3 个片段"
ffmpeg -f lavfi -i "sine=frequency=659:duration=1800" \
  -ar 44100 -b:a 128k -y "$TEST_DIR/test_30min.mp3" \
  -loglevel error

FILE_SIZE=$(du -h "$TEST_DIR/test_30min.mp3" | cut -f1)
echo "   ✓ 完成: $FILE_SIZE"
echo ""

# 4. 60分钟（切6片）
echo "4️⃣  生成 60分钟音频 (test_60min.mp3) - 将切成 6 个片段"
ffmpeg -f lavfi -i "sine=frequency=784:duration=3600" \
  -ar 44100 -b:a 128k -y "$TEST_DIR/test_60min.mp3" \
  -loglevel error

FILE_SIZE=$(du -h "$TEST_DIR/test_60min.mp3" | cut -f1)
echo "   ✓ 完成: $FILE_SIZE"
echo ""

# 5. 大文件测试（200MB）
echo "5️⃣  生成 200MB 音频 (test_200mb.mp3) - 测试大文件上传"
ffmpeg -f lavfi -i "sine=frequency=880:duration=4800" \
  -ar 44100 -b:a 320k -y "$TEST_DIR/test_200mb.mp3" \
  -loglevel error

FILE_SIZE=$(du -h "$TEST_DIR/test_200mb.mp3" | cut -f1)
echo "   ✓ 完成: $FILE_SIZE"
echo ""

# 显示结果
echo "================================"
echo "🎉 所有测试音频生成完成！"
echo ""
echo "📁 文件列表:"
ls -lh "$TEST_DIR" | grep -v "^d" | awk '{print "   " $9 " - " $5}'
echo ""
echo "💡 测试建议:"
echo "   1. 先用 test_15min.mp3 测试基本功能（2个片段）"
echo "   2. 用 test_30min.mp3 测试并发效果（3个片段，3个Worker）"
echo "   3. 用 test_60min.mp3 测试 Worker Pool 复用（6个片段）"
echo "   4. 用 test_200mb.mp3 测试大文件处理"
echo ""
echo "🚀 启动服务器: go run cmd/api/main.go"
echo "🌐 访问: http://localhost:8080"
echo ""
echo "📖 详细测试指南: 查看 TEST_GUIDE.md"
