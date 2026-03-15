#!/usr/bin/env bash
# Edge Proxy 交互式配置脚本
# 根据提示输入各项配置，生成 edge-proxy-config.yaml

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EDGE_PROXY_DIR="$(dirname "$SCRIPT_DIR")"
CONFIG_FILE="${EDGE_PROXY_DIR}/edge-proxy-config.yaml"
EXAMPLE_FILE="${EDGE_PROXY_DIR}/edge-proxy-config.yaml.example"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info() { echo -e "${CYAN}ℹ ${1}${NC}"; }
success() { echo -e "${GREEN}✓ ${1}${NC}"; }
warn() { echo -e "${YELLOW}⚠ ${1}${NC}"; }
err() { echo -e "${RED}✗ ${1}${NC}"; }

# 读取用户输入，支持默认值
# prompt_input "提示文字" "默认值" "变量名" [secure]
prompt_input() {
    local prompt="$1"
    local default="$2"
    local var_name="$3"
    local secure="${4:-false}"
    local value

    if [[ -n "$default" ]]; then
        if [[ "$secure" == "true" ]]; then
            read -r -s -p "${prompt} [输入时隐藏]: " value
            echo ""
            value="${value:-$default}"
        else
            read -r -p "${prompt} [${default}]: " value
            value="${value:-$default}"
        fi
    else
        while true; do
            if [[ "$secure" == "true" ]]; then
                read -r -s -p "${prompt}: " value
                echo ""
            else
                read -r -p "${prompt}: " value
            fi
            if [[ -n "$value" ]]; then
                break
            fi
            err "此项为必填，请重新输入"
        done
    fi
    eval "$var_name=\"$value\""
}

# 询问 Yes/No，默认 No
prompt_yn() {
    local prompt="$1"
    local default="${2:-n}"
    local value
    read -r -p "${prompt} [y/N]: " value
    value="${value:-$default}"
    local lower
    lower="$(echo "$value" | tr 'A-Z' 'a-z')"
    [[ "$lower" == "y" || "$lower" == "yes" ]]
}

echo ""
echo "=============================================="
echo "  Linkyun Edge Proxy 配置向导"
echo "=============================================="
echo ""

# 检查是否已存在配置
if [[ -f "$CONFIG_FILE" ]]; then
    if ! prompt_yn "配置文件已存在 (${CONFIG_FILE})，是否覆盖?" "n"; then
        info "已取消，配置未修改"
        exit 0
    fi
fi

# ========== 基础配置（必填） ==========
echo ""
info "【基础配置】"
echo "──────────────────────────────────────────────"

prompt_input "Linkyun Server 地址" "http://localhost:8080" SERVER_URL
prompt_input "Edge Token（从 Agent 编辑页复制）" "" EDGE_TOKEN
prompt_input "Agent UUID" "" AGENT_UUID

# ========== LLM 配置 ==========
echo ""
info "【LLM 配置】"
echo "──────────────────────────────────────────────"
echo "支持的 Provider 预设: openai, ollama, ollama-openai, deepseek, qwen, doubao, moonshot, zhipu, claude, gemini, ernie"
echo ""

# 第一个 LLM Provider
prompt_input "LLM Provider 类型" "openai" LLM_PROVIDER
prompt_input "Provider 名称（用于标识）" "${LLM_PROVIDER}-default" LLM_NAME

LLM_MODEL_DEFAULT="gpt-4o-mini"
case "$LLM_PROVIDER" in
    openai) LLM_MODEL_DEFAULT="gpt-4o-mini";;
    ollama|ollama-openai) LLM_MODEL_DEFAULT="llama3";;
    deepseek) LLM_MODEL_DEFAULT="deepseek-chat";;
    qwen) LLM_MODEL_DEFAULT="qwen-turbo";;
    doubao) LLM_MODEL_DEFAULT="doubao-pro-32k";;
    moonshot) LLM_MODEL_DEFAULT="moonshot-v1-8k";;
    zhipu) LLM_MODEL_DEFAULT="glm-4-flash";;
    claude) LLM_MODEL_DEFAULT="claude-sonnet-4-6";;
    gemini) LLM_MODEL_DEFAULT="gemini-2.0-flash";;
    ernie) LLM_MODEL_DEFAULT="ernie-4.0-8k";;
esac
prompt_input "模型名称" "$LLM_MODEL_DEFAULT" LLM_MODEL

# API Key（部分 provider 需要）
NEED_API_KEY=false
case "$LLM_PROVIDER" in
    openai|deepseek|qwen|doubao|moonshot|zhipu|claude|gemini|ernie) NEED_API_KEY=true;;
esac

LLM_API_KEY=""
LLM_BASE_URL=""
if $NEED_API_KEY; then
    prompt_input "API Key" "" LLM_API_KEY "true"
    # 对于 openai/ollama-openai，可能需要自定义 base_url
    if [[ "$LLM_PROVIDER" == "openai" || "$LLM_PROVIDER" == "ollama-openai" ]]; then
        default_base=""
        [[ "$LLM_PROVIDER" == "openai" ]] && default_base="https://api.openai.com/v1"
        [[ "$LLM_PROVIDER" == "ollama-openai" ]] && default_base="http://localhost:11434/v1"
        prompt_input "Base URL（可留空使用预设）" "$default_base" LLM_BASE_URL
    fi
else
    if [[ "$LLM_PROVIDER" == "ollama-openai" ]]; then
        prompt_input "Base URL" "http://localhost:11434/v1" LLM_BASE_URL
    fi
fi

prompt_input "Temperature (0-2)" "0.7" LLM_TEMPERATURE
prompt_input "Max Tokens" "4096" LLM_MAX_TOKENS

# ========== 可选配置 ==========
echo ""
info "【可选配置】"
echo "──────────────────────────────────────────────"

RULES_ENABLED=false
if prompt_yn "是否启用 Rules?" "n"; then
    RULES_ENABLED=true
    prompt_input "Rules 目录" "./rules" RULES_DIR
fi

SKILLS_ENABLED=false
if prompt_yn "是否启用 Skills?" "n"; then
    SKILLS_ENABLED=true
    prompt_input "Skills 目录" "./skills" SKILLS_DIR
fi

MCP_ENABLED=false
if prompt_yn "是否启用 MCP?" "n"; then
    MCP_ENABLED=true
fi

prompt_input "日志级别 (debug|info|warn|error)" "info" LOG_LEVEL
prompt_input "心跳间隔 (如 15s)" "15s" HEARTBEAT_INTERVAL
prompt_input "轮询超时 (如 30s)" "30s" POLL_TIMEOUT

# ========== 生成配置文件 ==========
echo ""
info "正在生成配置文件..."

# 转义 YAML 字符串值（统一加双引号并转义内部双引号）
escape_yaml() {
    local s="$1"
    echo "\"${s//\"/\\\"}\""
}

# 构建 LLM provider 的 YAML 块
build_llm_yaml() {
    local indent="$1"
    echo "${indent}- name: $(escape_yaml "$LLM_NAME")"
    echo "${indent}  provider: $(escape_yaml "$LLM_PROVIDER")"
    [[ -n "$LLM_BASE_URL" ]] && echo "${indent}  base_url: $(escape_yaml "$LLM_BASE_URL")"
    [[ -n "$LLM_API_KEY" ]] && echo "${indent}  api_key: $(escape_yaml "$LLM_API_KEY")"
    echo "${indent}  model: $(escape_yaml "$LLM_MODEL")"
    echo "${indent}  temperature: $LLM_TEMPERATURE"
    echo "${indent}  max_tokens: $LLM_MAX_TOKENS"
}

{
    echo "# Linkyun Edge Proxy 配置文件"
    echo "# 由 scripts/configure.sh 自动生成于 $(date '+%Y-%m-%d %H:%M:%S')"
    echo ""
    echo "# Linkyun Server 地址"
    echo "server_url: $(escape_yaml "$SERVER_URL")"
    echo ""
    echo "# Edge Token（从 Agent 编辑页复制）"
    echo "edge_token: $(escape_yaml "$EDGE_TOKEN")"
    echo ""
    echo "# Agent UUID"
    echo "agent_uuid: $(escape_yaml "$AGENT_UUID")"
    echo ""
    echo "# ============================================================"
    echo "# LLM 配置"
    echo "# ============================================================"
    echo "llm:"
    echo "  default: $(escape_yaml "$LLM_NAME")"
    echo "  providers:"
    build_llm_yaml "    "

    if $RULES_ENABLED; then
        echo ""
        echo "# ============================================================"
        echo "# Rules 配置"
        echo "# ============================================================"
        echo "rules:"
        echo "  enabled: true"
        echo "  directories:"
        echo "    - $(escape_yaml "$RULES_DIR")"
    fi

    if $SKILLS_ENABLED; then
        echo ""
        echo "# ============================================================"
        echo "# Skills 配置"
        echo "# ============================================================"
        echo "skills:"
        echo "  enabled: true"
        echo "  directory: $(escape_yaml "$SKILLS_DIR")"
    fi

    if $MCP_ENABLED; then
        echo ""
        echo "# ============================================================"
        echo "# MCP 配置"
        echo "# ============================================================"
        echo "mcp:"
        echo "  enabled: true"
        echo "  servers: []  # 请参考 edge-proxy-config.yaml.example 添加 MCP 服务器"
    fi

    echo ""
    echo "# 心跳与轮询"
    echo "heartbeat_interval: $HEARTBEAT_INTERVAL"
    echo "poll_timeout: $POLL_TIMEOUT"
    echo ""
    echo "# 日志级别"
    echo "log_level: $(escape_yaml "$LOG_LEVEL")"
} > "$CONFIG_FILE"

success "配置文件已生成: $CONFIG_FILE"
echo ""
info "下一步："
echo "  1. 检查并编辑 $CONFIG_FILE（如需添加更多 LLM Provider 或 MCP 服务器）"
echo "  2. 运行 edge-proxy 启动服务"
echo ""
