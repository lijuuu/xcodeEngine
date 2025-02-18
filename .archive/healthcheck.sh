#!/bin/bash
set -e

# Check if all required runtimes are available
check_runtimes() {
    local exit_code=0

    # Check Python
    if ! python3 -c "print('ok')" &> /dev/null; then
        echo "Python runtime check failed"
        exit_code=1
    fi

    # Check Node.js
    if ! node -e "console.log('ok')" &> /dev/null; then
        echo "Node.js runtime check failed"
        exit_code=1
    fi

    # Check Go
    if ! go version &> /dev/null; then
        echo "Go runtime check failed"
        exit_code=1
    fi

    # Check C++
    echo 'int main() { return 0; }' > /app/temp/test.cpp
    if ! g++ -std=c++17 /app/temp/test.cpp -o /app/temp/test 2>/dev/null; then
        echo "C++ runtime check failed"
        exit_code=1
    else
        rm -f /app/temp/test
    fi
    rm -f /app/temp/test.cpp

    return $exit_code
}

# Check if temp directory is writable
check_permissions() {
    if ! touch /app/temp/test_file 2>/dev/null; then
        echo "Temp directory not writable"
        return 1
    fi
    rm -f /app/temp/test_file
    return 0
}

# Run all checks
main() {
    local failed=0

    check_runtimes || failed=1
    check_permissions || failed=1

    return $failed
}

main