#!/bin/bash

# Demo script for podcast-tui

echo "Building podcast-tui..."
go build -o podcast-tui ./cmd/podcast-tui

echo "Creating demo config..."
mkdir -p ~/.config/podcast-tui

cat > ~/.config/podcast-tui/subscriptions.json << EOF
{
  "podcasts": [
    {
      "Title": "Go Time",
      "URL": "https://changelog.com/gotime/feed",
      "Description": "A weekly panelist podcast discussing the Go programming language, the community, and everything in between."
    },
    {
      "Title": "The Changelog",
      "URL": "https://changelog.com/podcast/feed",
      "Description": "Conversations with the hackers, leaders, and innovators of the software world."
    }
  ]
}
EOF

echo "Demo config created. You can now run ./podcast-tui"
echo ""
echo "Quick start:"
echo "  - Use 'j/k' to navigate up/down"
echo "  - Use 'l' to enter a podcast and see episodes"
echo "  - Press Enter on an episode to play it (requires mpv)"
echo "  - Use ':add <url>' to add new podcasts"
echo "  - Press 'q' to quit"