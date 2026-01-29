#!/usr/bin/env bash
# Fetch new user feedback from production and write to FEEDBACK.md.
#
# Reads secrets from .env.feedback (gitignored):
#   PROD_HOST=https://subtitler.pottingers.us
#   API_SESSION_ID=<session cookie value>
#
# Uses .feedback-cursor (gitignored) to track the last-fetched timestamp
# so repeated runs only fetch new items.
#
# Exit codes:
#   0 - New feedback written to FEEDBACK.md
#   1 - No new feedback found
#   2 - Error (auth failure, network, missing config)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

ENV_FILE="$PROJECT_DIR/.env.feedback"
CURSOR_FILE="$PROJECT_DIR/.feedback-cursor"
OUTPUT_FILE="$PROJECT_DIR/FEEDBACK.md"

# Load secrets
if [ ! -f "$ENV_FILE" ]; then
    echo "Error: $ENV_FILE not found." >&2
    echo "Create it with:" >&2
    echo "  PROD_HOST=https://subtitler.pottingers.us" >&2
    echo "  API_SESSION_ID=<your session cookie>" >&2
    exit 2
fi

# shellcheck source=/dev/null
source "$ENV_FILE"

if [ -z "${PROD_HOST:-}" ] || [ -z "${API_SESSION_ID:-}" ]; then
    echo "Error: PROD_HOST and API_SESSION_ID must be set in $ENV_FILE" >&2
    exit 2
fi

# Build query URL
URL="${PROD_HOST}/api/admin/feedback?status=new&limit=50"

# Add cursor if exists
if [ -f "$CURSOR_FILE" ]; then
    AFTER=$(cat "$CURSOR_FILE")
    URL="${URL}&after=${AFTER}"
fi

# Fetch feedback
HTTP_RESPONSE=$(curl -s -w "\n%{http_code}" \
    -H "Authorization: Bearer ${API_SESSION_ID}" \
    "$URL")

HTTP_BODY=$(echo "$HTTP_RESPONSE" | sed '$d')
HTTP_CODE=$(echo "$HTTP_RESPONSE" | tail -1)

if [ "$HTTP_CODE" = "401" ]; then
    echo "Error: Authentication failed (401). Update API_SESSION_ID in $ENV_FILE" >&2
    exit 2
fi

if [ "$HTTP_CODE" != "200" ]; then
    echo "Error: API returned HTTP $HTTP_CODE" >&2
    echo "$HTTP_BODY" >&2
    exit 2
fi

# Parse response - extract feedback count
FEEDBACK_COUNT=$(echo "$HTTP_BODY" | python3 -c "
import json, sys
data = json.load(sys.stdin)
print(len(data.get('feedback', []) or []))
")

if [ "$FEEDBACK_COUNT" = "0" ]; then
    echo "No new feedback found."
    exit 1
fi

# Format feedback as markdown and find newest timestamp
echo "$HTTP_BODY" | python3 -c "
import json, sys

data = json.load(sys.stdin)
feedback_items = data.get('feedback', []) or []

lines = []
newest_ts = ''
for item in feedback_items:
    fb_type = item.get('feedback_type', 'general')
    text = item.get('feedback_text', '').replace('\n', ' ').strip()
    created = item.get('created_at', '')
    page = item.get('page_url', '')
    rating = item.get('rating')

    parts = [f'[{fb_type}]']
    if rating:
        parts.append(f'Rating: {rating}/5')
    parts.append(f'- \"{text}\"')
    if created:
        date_part = created[:10] if len(created) >= 10 else created
        parts.append(f'({date_part}')
        if page:
            parts.append(f', page: {page})')
        else:
            parts.append(')')
    line = '* ' + ' '.join(parts)
    lines.append(line)

    if created > newest_ts:
        newest_ts = created

for line in lines:
    print(line)

# Print cursor to stderr so we can capture it separately
print(newest_ts, file=sys.stderr)
" > "$PROJECT_DIR/.feedback-tmp" 2> "$PROJECT_DIR/.feedback-cursor-tmp"

# Write to FEEDBACK.md
if [ -f "$OUTPUT_FILE" ]; then
    # Append to existing
    cat "$PROJECT_DIR/.feedback-tmp" >> "$OUTPUT_FILE"
else
    cat "$PROJECT_DIR/.feedback-tmp" > "$OUTPUT_FILE"
fi

# Update cursor
mv "$PROJECT_DIR/.feedback-cursor-tmp" "$CURSOR_FILE"
rm -f "$PROJECT_DIR/.feedback-tmp"

echo "Wrote $FEEDBACK_COUNT feedback item(s) to FEEDBACK.md"
exit 0
