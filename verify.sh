#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[TEST]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

log "Build the binary..."
go build -o icopy main.go

log "Cleaning up old test data..."
rm -rf test_src test_dst badger custom.log .file_status.txt

log "Creating test source directory..."
mkdir -p test_src/subdir

# 1. Create Small Image (JPG) - Should be copied by -image
log "Generating small.jpg..."
echo "fake image content" > test_src/small.jpg
touch -t 202301011200 test_src/small.jpg

# 2. Create Small Video (MP4) - Should be copied by -video
log "Generating small.mp4..."
echo "fake video content" > test_src/small.mp4
touch -t 202302011200 test_src/small.mp4

# 3. Create Large Video (MP4) - Should be copied by -video and trigger fast-hash
log "Generating large.mp4 (>50MB)..."
# Create a 55MB file using dd. 
# if /dev/zero is efficient, this is fast.
dd if=/dev/zero of=test_src/large.mp4 bs=1M count=55 status=none
touch -t 202303011200 test_src/large.mp4

# 4. Create MPG Video - Should be copied by -video (fallback to mod time)
log "Generating old.mpg..."
echo "fake mpg content" > test_src/old.mpg
touch -t 202304011200 test_src/old.mpg

# 5. Create Nested File - Should be found by recursive scan
log "Generating nested.jpg..."
echo "nested content" > test_src/subdir/nested.jpg
touch -t 202305011200 test_src/subdir/nested.jpg


log "--- Scenario 1: Scanning Files ---"
mkdir -p test_dst
./icopy -in test_src -out test_dst -scan -recursive


log "--- Scenario 2: Copying Images (Recursive) ---"
./icopy -in test_src -out test_dst -image -recursive -workers 5 -dirformat DATE
# Expect: small.jpg (2023-01), nested.jpg (2023-05)
# Check if destination folders exist
if [ ! -f "test_dst/2023-01-01/small.jpg" ]; then error "small.jpg not copied correctly"; fi
if [ ! -f "test_dst/2023-05-01/nested.jpg" ]; then error "nested.jpg not copied correctly"; fi

log "--- Scenario 3: Copying Videos (Recursive, Fast Hash) ---"
./icopy -in test_src -out test_dst -video -recursive -fast-hash -workers 5 -dirformat DATE
# Expect: small.mp4 (2023-02), large.mp4 (2023-03), old.mpg (2023-04)
if [ ! -f "test_dst/2023-02-01/small.mp4" ]; then error "small.mp4 not copied correctly"; fi
if [ ! -f "test_dst/2023-03-01/large.mp4" ]; then error "large.mp4 not copied correctly"; fi
if [ ! -f "test_dst/2023-04-01/old.mpg" ]; then error "old.mpg not copied correctly"; fi

log "--- Scenario 4: Force Copy (Overwrite) ---"
# Modify source file to verify overwrite happens
echo "modified content" > test_src/small.mp4
touch -t 202302011200 test_src/small.mp4
# Run with -force
./icopy -in test_src -out test_dst -video -recursive -force -dirformat DATE
# Verify content
if grep -q "modified content" test_dst/2023-02-01/small.mp4; then
    log "Force copy verification passed (content updated)"
else
    error "Force copy failed: Content was not updated"
fi

log "--- Scenario 5: Check Spinner output (Manual Check) ---"
# We can't easily check spinner in script, but the run above should have shown it.

log "--- Cleanup ---"
rm -rf test_src test_dst badger custom.log .file_status.txt

log "All verification scenarios passed successfully!"
