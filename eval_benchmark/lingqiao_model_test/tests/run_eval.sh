#!/usr/bin/env bash
# ============================================================================
# run_eval.sh — 灵桥评测自动化评分脚本
#
# 用法:
#   bash run_eval.sh --target starter/        # 评测初始代码（预期不通过）
#   bash run_eval.sh --target reference/      # 评测参考解答（预期满分）
#
# 输出: result.json
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BASE_DIR="$(dirname "$SCRIPT_DIR")"
TARGET_DIR=""
RESULT_FILE="$BASE_DIR/result.json"

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --target)
            TARGET_DIR="$BASE_DIR/$2"
            shift 2
            ;;
        *)
            echo "Usage: $0 --target <starter/|reference/>"
            exit 1
            ;;
    esac
done

if [[ -z "$TARGET_DIR" ]]; then
    echo "Error: --target is required"
    echo "Usage: $0 --target <starter/|reference/>"
    exit 1
fi

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "============================================"
echo " Lingqiao Model Test - Evaluation Runner"
echo "============================================"
echo "Target: $TARGET_DIR"
echo ""

# ============================================================================
# Task 1: C++ Hook 引擎评测
# ============================================================================
echo -e "${YELLOW}[Task 1] C++ Hook Engine & Pattern Scan${NC}"

TASK1_SCORE=0
TASK1_MAX=100
TASK1_COMPILE=false
TASK1_TESTS_PASSED=0
TASK1_TESTS_TOTAL=10
TASK1_SANITIZER=false
TASK1_ERRORS=()

C_HOOK_DIR=""
if [[ -d "$TARGET_DIR/c_hook_solution" ]]; then
    C_HOOK_DIR="$TARGET_DIR/c_hook_solution"
elif [[ -d "$TARGET_DIR/c_hook" ]]; then
    C_HOOK_DIR="$TARGET_DIR/c_hook"
fi

if [[ -n "$C_HOOK_DIR" && -d "$C_HOOK_DIR" ]]; then
    # 创建临时构建目录
    BUILD_DIR=$(mktemp -d)
    trap "rm -rf $BUILD_DIR" EXIT

    # 复制源文件到构建目录
    cp "$C_HOOK_DIR"/*.{h,c,cpp} "$BUILD_DIR/" 2>/dev/null || true
    cp "$SCRIPT_DIR/test_pattern_scan.cpp" "$BUILD_DIR/" 2>/dev/null || true
    cp "$SCRIPT_DIR/test_hook_engine.cpp" "$BUILD_DIR/" 2>/dev/null || true
    cp "$SCRIPT_DIR/test_bridge_protocol.cpp" "$BUILD_DIR/" 2>/dev/null || true

    cd "$BUILD_DIR"

    # 编译 (20 分)
    echo -n "  Compiling... "
    COMPILE_CMD="g++ -std=c++17 -Wall -Wextra -Werror -c pattern_scan.cpp hook_engine.c v8_intercept.cpp -I. 2>&1"
    if COMPILE_OUT=$(eval $COMPILE_CMD); then
        echo -e "${GREEN}OK${NC}"
        TASK1_COMPILE=true
        TASK1_SCORE=$((TASK1_SCORE + 20))
    else
        echo -e "${RED}FAILED${NC}"
        TASK1_ERRORS+=("Compilation failed: $COMPILE_OUT")
    fi

    # 单元测试 (50 分)
    if $TASK1_COMPILE; then
        echo "  Running tests..."

        # test_pattern_scan
        if g++ -std=c++17 -o test_ps test_pattern_scan.cpp pattern_scan.cpp -I. -lpthread 2>/dev/null; then
            if OUTPUT=$(./test_ps 2>&1); then
                PASSED=$(echo "$OUTPUT" | grep -oP '\d+(?= passed)' || echo "0")
                TASK1_TESTS_PASSED=$((TASK1_TESTS_PASSED + PASSED))
                echo "    pattern_scan: $PASSED tests passed"
            fi
        fi

        # test_hook_engine
        if g++ -std=c++17 -o test_he test_hook_engine.cpp hook_engine.c -I. -lpthread 2>/dev/null; then
            if OUTPUT=$(./test_he 2>&1); then
                PASSED=$(echo "$OUTPUT" | grep -oP '\d+(?= passed)' || echo "0")
                TASK1_TESTS_PASSED=$((TASK1_TESTS_PASSED + PASSED))
                echo "    hook_engine: $PASSED tests passed"
            fi
        fi

        TASK1_SCORE=$((TASK1_SCORE + TASK1_TESTS_PASSED * 5))
        if [[ $TASK1_SCORE -gt 70 ]]; then
            TASK1_SCORE=70
        fi

        # Sanitizer 检测 (30 分)
        echo -n "  Running sanitizers... "
        TSAN_PASS=true
        if g++ -std=c++17 -fsanitize=thread -o test_he_tsan test_hook_engine.cpp hook_engine.c -I. -lpthread 2>/dev/null; then
            if ! ./test_he_tsan 2>&1 | grep -q "WARNING: ThreadSanitizer"; then
                echo -e "${GREEN}TSan clean${NC}"
            else
                TSAN_PASS=false
                TASK1_ERRORS+=("TSan: data race detected")
                echo -e "${RED}TSan violations${NC}"
            fi
        fi

        ASAN_PASS=true
        if g++ -std=c++17 -fsanitize=address -o test_ps_asan test_pattern_scan.cpp pattern_scan.cpp -I. -lpthread 2>/dev/null; then
            if ! ./test_ps_asan 2>&1 | grep -q "ERROR: AddressSanitizer"; then
                echo -e "  ${GREEN}ASan clean${NC}"
            else
                ASAN_PASS=false
                TASK1_ERRORS+=("ASan: memory error detected")
            fi
        fi

        if $TSAN_PASS && $ASAN_PASS; then
            TASK1_SANITIZER=true
            TASK1_SCORE=$((TASK1_SCORE + 30))
        fi
    fi
else
    TASK1_ERRORS+=("C++ source directory not found")
fi

echo "  Score: $TASK1_SCORE / $TASK1_MAX"
echo ""

# ============================================================================
# Task 2: Go Server 评测
# ============================================================================
echo -e "${YELLOW}[Task 2] Go Agent Server${NC}"

TASK2_SCORE=0
TASK2_MAX=100
TASK2_COMPILE=false
TASK2_TESTS_PASSED=0
TASK2_TESTS_TOTAL=10
TASK2_RACE=false
TASK2_ERRORS=()

GO_DIR=""
if [[ -d "$TARGET_DIR/go_server_solution" ]]; then
    GO_DIR="$TARGET_DIR/go_server_solution"
elif [[ -d "$TARGET_DIR/go_server" ]]; then
    GO_DIR="$TARGET_DIR/go_server"
fi

if [[ -n "$GO_DIR" && -d "$GO_DIR" ]]; then
    # 复制测试文件
    cp "$SCRIPT_DIR/server_test.go" "$GO_DIR/" 2>/dev/null || true

    cd "$GO_DIR"

    # 编译 (20 分)
    echo -n "  Compiling... "
    if go build ./... 2>/dev/null; then
        echo -e "${GREEN}OK${NC}"
        TASK2_COMPILE=true
        TASK2_SCORE=$((TASK2_SCORE + 20))
    else
        echo -e "${RED}FAILED${NC}"
        TASK2_ERRORS+=("go build failed")
    fi

    # 单元测试 (50 分)
    if $TASK2_COMPILE; then
        echo -n "  Running tests... "
        TEST_OUTPUT=$(go test -v -count=1 ./... 2>&1 || true)
        TASK2_TESTS_PASSED=$(echo "$TEST_OUTPUT" | grep -c "^--- PASS" || echo "0")
        TESTS_FAILED=$(echo "$TEST_OUTPUT" | grep -c "^--- FAIL" || echo "0")
        echo "$TASK2_TESTS_PASSED passed, $TESTS_FAILED failed"
        TASK2_SCORE=$((TASK2_SCORE + TASK2_TESTS_PASSED * 5))
        if [[ $TASK2_SCORE -gt 70 ]]; then
            TASK2_SCORE=70
        fi

        # Race 检测 (30 分)
        echo -n "  Running race detector... "
        if RACE_OUTPUT=$(go test -race -count=1 ./... 2>&1); then
            echo -e "${GREEN}clean${NC}"
            TASK2_RACE=true
            TASK2_SCORE=$((TASK2_SCORE + 30))
        else
            if echo "$RACE_OUTPUT" | grep -q "DATA RACE"; then
                echo -e "${RED}races detected${NC}"
                TASK2_ERRORS+=("Data race detected")
            else
                echo -e "${RED}test failures${NC}"
                TASK2_ERRORS+=("Tests failed under race detector")
            fi
        fi
    fi

    # 清理测试文件
    rm -f "$GO_DIR/server_test.go" 2>/dev/null || true
else
    TASK2_ERRORS+=("Go source directory not found")
fi

echo "  Score: $TASK2_SCORE / $TASK2_MAX"
echo ""

# ============================================================================
# Task 3: Bridge Protocol 评测 (合并 C++ 和 Go 的加解密互通测试)
# ============================================================================
echo -e "${YELLOW}[Task 3] Bridge Protocol${NC}"

TASK3_SCORE=0
TASK3_MAX=100
TASK3_COMPILE=true
TASK3_COMPAT=false
TASK3_ERRORS=()

# C++ 端 bridge protocol 测试
if [[ -n "$C_HOOK_DIR" && -d "$C_HOOK_DIR" ]]; then
    cd "$BUILD_DIR" 2>/dev/null || true
    echo -n "  Compiling bridge protocol test... "
    if g++ -std=c++17 -o test_bp test_bridge_protocol.cpp v8_intercept.cpp pattern_scan.cpp -I. -lpthread 2>/dev/null; then
        echo -e "${GREEN}OK${NC}"
        TASK3_SCORE=$((TASK3_SCORE + 10))

        echo -n "  Running bridge tests... "
        if BP_OUTPUT=$(./test_bp 2>&1); then
            BP_PASSED=$(echo "$BP_OUTPUT" | grep -oP '\d+(?= passed)' || echo "0")
            BP_FAILED=$(echo "$BP_OUTPUT" | grep -oP '\d+(?= failed)' || echo "0")
            echo "$BP_PASSED passed, $BP_FAILED failed"
            if [[ "$BP_FAILED" == "0" ]]; then
                TASK3_SCORE=$((TASK3_SCORE + 40))
                TASK3_COMPAT=true
            else
                TASK3_SCORE=$((TASK3_SCORE + BP_PASSED * 5))
            fi
        else
            echo -e "${RED}CRASHED${NC}"
            TASK3_ERRORS+=("Bridge protocol test crashed")
        fi
    else
        echo -e "${RED}FAILED${NC}"
        TASK3_COMPILE=false
        TASK3_ERRORS+=("Bridge protocol compilation failed")
    fi
fi

# Go 端 bridge 加解密在 Task 2 的测试中已覆盖
# 额外加分: 如果 Go 的 EncryptBridgeMessage/DecryptBridgeMessage 测试通过
if [[ -n "$GO_DIR" && -d "$GO_DIR" ]] && $TASK2_COMPILE; then
    cd "$GO_DIR"
    if go test -run "TestBridge" -v -count=1 ./... 2>&1 | grep -q "PASS"; then
        TASK3_SCORE=$((TASK3_SCORE + 30))
    fi
fi

# 错误处理评分
if $TASK3_COMPAT; then
    TASK3_SCORE=$((TASK3_SCORE + 20))
fi

if [[ $TASK3_SCORE -gt 100 ]]; then
    TASK3_SCORE=100
fi

echo "  Score: $TASK3_SCORE / $TASK3_MAX"
echo ""

# ============================================================================
# 汇总输出
# ============================================================================
TOTAL_SCORE=$((TASK1_SCORE + TASK2_SCORE + TASK3_SCORE))
TOTAL_MAX=$((TASK1_MAX + TASK2_MAX + TASK3_MAX))

echo "============================================"
echo -e " ${YELLOW}TOTAL SCORE: $TOTAL_SCORE / $TOTAL_MAX${NC}"
echo "============================================"

# 生成 JSON 报告
cat > "$RESULT_FILE" <<EOF
{
  "total_score": $TOTAL_SCORE,
  "max_score": $TOTAL_MAX,
  "timestamp": "$(date -Iseconds)",
  "target": "$TARGET_DIR",
  "tasks": [
    {
      "name": "task1_c_hook",
      "score": $TASK1_SCORE,
      "max": $TASK1_MAX,
      "compile": $TASK1_COMPILE,
      "tests_passed": $TASK1_TESTS_PASSED,
      "tests_total": $TASK1_TESTS_TOTAL,
      "sanitizer_clean": $TASK1_SANITIZER
    },
    {
      "name": "task2_go_server",
      "score": $TASK2_SCORE,
      "max": $TASK2_MAX,
      "compile": $TASK2_COMPILE,
      "tests_passed": $TASK2_TESTS_PASSED,
      "tests_total": $TASK2_TESTS_TOTAL,
      "race_clean": $TASK2_RACE
    },
    {
      "name": "task3_bridge_protocol",
      "score": $TASK3_SCORE,
      "max": $TASK3_MAX,
      "compile": $TASK3_COMPILE,
      "protocol_compat": $TASK3_COMPAT
    }
  ]
}
EOF

echo ""
echo "Report saved to: $RESULT_FILE"
