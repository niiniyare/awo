#!/bin/bash
echo "AMIS Admin - Project Structure"
echo "=============================="
echo ""
find . -not -path './sdk/*' -not -path './.git/*' -not -name '.*' | sort | head -30
echo ""
echo "SDK files: $(ls sdk/ | wc -l) files in sdk/"
echo ""
echo "To run:"
echo "  http-server -p 8080"
echo "  Open http://localhost:8080/pages/"
echo ""
echo "API base URL configured in pages/index.html (default: http://localhost:3000/api)"
echo "Expected response format: {status: 0, data: {...}, msg: ''}"
