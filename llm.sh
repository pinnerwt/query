#!/bin/bash
# Start llama.cpp server for qwen3.5-27b normalization
# Used by: ./ocr --normalize-url http://127.0.0.1:8090

exec ~/builds/llama.cpp/build/bin/llama-server \
  -m ~/builds/llama.cpp/models/qwen3.5-27b \
  -c 16384 \
  --n-gpu-layers 999 \
  -b 4096 \
  -ub 2048 \
  --flash-attn on \
  --cache-type-k q8_0 \
  --cache-type-v q8_0 \
  --port 8090
