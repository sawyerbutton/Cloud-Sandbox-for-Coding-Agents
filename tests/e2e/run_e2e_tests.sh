#!/bin/bash

# End-to-End Integration Tests for Cloud Sandbox
# This script tests the complete flow from Gateway through to sandbox execution

set -e

# Configuration
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
TEST_USER_ID="${TEST_USER_ID:-e2e-test-user}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counters
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
log_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

check_response() {
    local response="$1"
    local expected_field="$2"
    local test_name="$3"

    if echo "$response" | jq -e ".$expected_field" > /dev/null 2>&1; then
        log_success "$test_name"
        return 0
    else
        log_error "$test_name - Expected field '$expected_field' not found in response"
        echo "Response: $response"
        return 1
    fi
}

check_status_code() {
    local status_code="$1"
    local expected="$2"
    local test_name="$3"

    if [ "$status_code" -eq "$expected" ]; then
        log_success "$test_name"
        return 0
    else
        log_error "$test_name - Expected $expected, got $status_code"
        return 1
    fi
}

# Ensure NO_PROXY is set to avoid proxy issues with localhost
export NO_PROXY=localhost,127.0.0.1

echo "========================================"
echo "Cloud Sandbox End-to-End Tests"
echo "========================================"
echo "Gateway URL: $GATEWAY_URL"
echo "Test User: $TEST_USER_ID"
echo ""

# Test 1: Health Check
log_info "Test 1: Health Check"
HEALTH_RESPONSE=$(curl -s "$GATEWAY_URL/health")
check_response "$HEALTH_RESPONSE" "status" "Health endpoint returns status"

# Test 2: Get Authentication Token
log_info "Test 2: Get Authentication Token"
TOKEN_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/auth/token" \
    -H "Content-Type: application/json" \
    -d "{\"user_id\": \"$TEST_USER_ID\", \"role\": \"user\"}")

ACCESS_TOKEN=$(echo "$TOKEN_RESPONSE" | jq -r '.access_token')
if [ -n "$ACCESS_TOKEN" ] && [ "$ACCESS_TOKEN" != "null" ]; then
    log_success "Token generation successful"
else
    log_error "Token generation failed"
    echo "Response: $TOKEN_RESPONSE"
    exit 1
fi

# Test 3: Create Session
log_info "Test 3: Create Session"
SESSION_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/sessions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -d '{"image": "python:3.11-slim", "cpu_count": 2, "memory_mb": 1024}')

SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.id')
if [ -n "$SESSION_ID" ] && [ "$SESSION_ID" != "null" ]; then
    log_success "Session created: $SESSION_ID"
else
    log_error "Session creation failed"
    echo "Response: $SESSION_RESPONSE"
fi

# Test 4: List Sessions
log_info "Test 4: List Sessions"
SESSIONS_RESPONSE=$(curl -s -X GET "$GATEWAY_URL/api/v1/sessions" \
    -H "Authorization: Bearer $ACCESS_TOKEN")

SESSION_COUNT=$(echo "$SESSIONS_RESPONSE" | jq '.sessions | length')
if [ "$SESSION_COUNT" -ge 1 ]; then
    log_success "List sessions returned $SESSION_COUNT session(s)"
else
    log_error "List sessions failed or empty"
    echo "Response: $SESSIONS_RESPONSE"
fi

# Test 5: Acquire Sandbox
log_info "Test 5: Acquire Sandbox"
SANDBOX_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/sandbox/acquire" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $ACCESS_TOKEN")

SANDBOX_ID=$(echo "$SANDBOX_RESPONSE" | jq -r '.sandbox_id')
if [ -n "$SANDBOX_ID" ] && [ "$SANDBOX_ID" != "null" ]; then
    log_success "Sandbox acquired: $SANDBOX_ID"
else
    log_error "Sandbox acquisition failed"
    echo "Response: $SANDBOX_RESPONSE"
fi

# Test 6: Execute Python Code
log_info "Test 6: Execute Python Code"
EXEC_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/execute" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -d "{\"sandbox_id\": \"$SANDBOX_ID\", \"code\": \"print('Hello from E2E test!')\", \"language\": \"python\"}")

EXIT_CODE=$(echo "$EXEC_RESPONSE" | jq -r '.exit_code')
STDOUT=$(echo "$EXEC_RESPONSE" | jq -r '.stdout')
if [ "$EXIT_CODE" = "0" ] && echo "$STDOUT" | grep -q "Hello from E2E test"; then
    log_success "Python code execution successful"
else
    log_error "Python code execution failed"
    echo "Response: $EXEC_RESPONSE"
fi

# Test 7: Execute Shell Command
log_info "Test 7: Execute Shell Command"
EXEC_CMD_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/execute" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -d "{\"sandbox_id\": \"$SANDBOX_ID\", \"command\": [\"echo\", \"Shell command works\"]}")

EXIT_CODE=$(echo "$EXEC_CMD_RESPONSE" | jq -r '.exit_code')
STDOUT=$(echo "$EXEC_CMD_RESPONSE" | jq -r '.stdout')
if [ "$EXIT_CODE" = "0" ] && echo "$STDOUT" | grep -q "Shell command works"; then
    log_success "Shell command execution successful"
else
    log_error "Shell command execution failed"
    echo "Response: $EXEC_CMD_RESPONSE"
fi

# Test 8: Write File
log_info "Test 8: Write File"
WRITE_RESPONSE=$(curl -s -X PUT "$GATEWAY_URL/api/v1/files" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -d "{\"sandbox_id\": \"$SANDBOX_ID\", \"path\": \"/workspace/test.txt\", \"content\": \"Hello from file!\"}")

if echo "$WRITE_RESPONSE" | jq -e '.success' > /dev/null 2>&1; then
    log_success "File write successful"
else
    log_error "File write failed"
    echo "Response: $WRITE_RESPONSE"
fi

# Test 9: List Files
log_info "Test 9: List Files"
FILES_RESPONSE=$(curl -s -X GET "$GATEWAY_URL/api/v1/files?sandbox_id=$SANDBOX_ID&path=/workspace" \
    -H "Authorization: Bearer $ACCESS_TOKEN")

if echo "$FILES_RESPONSE" | jq -e '.files' > /dev/null 2>&1; then
    FILE_COUNT=$(echo "$FILES_RESPONSE" | jq '.files | length')
    log_success "List files returned $FILE_COUNT file(s)"
else
    log_error "List files failed"
    echo "Response: $FILES_RESPONSE"
fi

# Test 10: Release Sandbox
log_info "Test 10: Release Sandbox"
RELEASE_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/sandbox/release" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -d "{\"sandbox_id\": \"$SANDBOX_ID\"}")

if echo "$RELEASE_RESPONSE" | jq -e '.success' > /dev/null 2>&1; then
    log_success "Sandbox released successfully"
else
    log_error "Sandbox release failed"
    echo "Response: $RELEASE_RESPONSE"
fi

# Test 11: Get Sandbox Stats
log_info "Test 11: Get Sandbox Stats"
STATS_RESPONSE=$(curl -s -X GET "$GATEWAY_URL/api/v1/sandbox/stats" \
    -H "Authorization: Bearer $ACCESS_TOKEN")

if echo "$STATS_RESPONSE" | jq -e '.total' > /dev/null 2>&1; then
    TOTAL=$(echo "$STATS_RESPONSE" | jq '.total')
    AVAILABLE=$(echo "$STATS_RESPONSE" | jq '.available')
    log_success "Sandbox stats: Total=$TOTAL, Available=$AVAILABLE"
else
    log_error "Sandbox stats failed"
    echo "Response: $STATS_RESPONSE"
fi

# Test 12: Pause Session
log_info "Test 12: Pause Session"
if [ -n "$SESSION_ID" ] && [ "$SESSION_ID" != "null" ]; then
    PAUSE_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/sessions/$SESSION_ID/pause" \
        -H "Authorization: Bearer $ACCESS_TOKEN")

    PAUSE_STATUS=$(echo "$PAUSE_RESPONSE" | jq -r '.status')
    if [ "$PAUSE_STATUS" = "paused" ]; then
        log_success "Session paused successfully"
    else
        log_error "Session pause failed"
        echo "Response: $PAUSE_RESPONSE"
    fi
else
    log_error "No session ID available for pause test"
fi

# Test 13: Resume Session
log_info "Test 13: Resume Session"
if [ -n "$SESSION_ID" ] && [ "$SESSION_ID" != "null" ]; then
    RESUME_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/sessions/$SESSION_ID/resume" \
        -H "Authorization: Bearer $ACCESS_TOKEN")

    RESUME_STATUS=$(echo "$RESUME_RESPONSE" | jq -r '.status')
    if [ "$RESUME_STATUS" = "active" ]; then
        log_success "Session resumed successfully"
    else
        log_error "Session resume failed"
        echo "Response: $RESUME_RESPONSE"
    fi
else
    log_error "No session ID available for resume test"
fi

# Test 14: Delete Session
log_info "Test 14: Delete Session"
if [ -n "$SESSION_ID" ] && [ "$SESSION_ID" != "null" ]; then
    DELETE_RESPONSE=$(curl -s -X DELETE "$GATEWAY_URL/api/v1/sessions/$SESSION_ID" \
        -H "Authorization: Bearer $ACCESS_TOKEN")

    if echo "$DELETE_RESPONSE" | jq -e '.success' > /dev/null 2>&1; then
        log_success "Session deleted successfully"
    else
        log_error "Session deletion failed"
        echo "Response: $DELETE_RESPONSE"
    fi
else
    log_error "No session ID available for delete test"
fi

# Test 15: Verify Unauthorized Access
log_info "Test 15: Verify Unauthorized Access"
UNAUTH_RESPONSE=$(curl -s -w "\n%{http_code}" -X GET "$GATEWAY_URL/api/v1/sessions")
UNAUTH_STATUS=$(echo "$UNAUTH_RESPONSE" | tail -n1)
if [ "$UNAUTH_STATUS" = "401" ]; then
    log_success "Unauthorized access correctly rejected with 401"
else
    log_error "Expected 401 for unauthorized access, got $UNAUTH_STATUS"
fi

# Summary
echo ""
echo "========================================"
echo "Test Summary"
echo "========================================"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
else
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
fi
