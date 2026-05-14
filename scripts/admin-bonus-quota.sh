#!/usr/bin/env bash
# Grant, revoke, or reset bonus submission quota for all participants of a lab.
#
# Examples:
#   scripts/admin-bonus-quota.sh --project colab-2026-p2 --add 5
#   scripts/admin-bonus-quota.sh --project colab-2026-p2 --add -2
#   scripts/admin-bonus-quota.sh --project colab-2026-p2 --reset
#
# Environment:
#   LABKIT_API_URL     defaults to http://localhost:8080
#   LABKIT_ADMIN_TOKEN bearer token; required
#   ASSUME_YES         set to 1 to skip the interactive confirmation

set -euo pipefail

PROJECT=""
DELTA=""
RESET=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --project)
      PROJECT="$2"; shift 2 ;;
    --add)
      DELTA="$2"; shift 2 ;;
    --reset)
      RESET=1; shift ;;
    -h|--help)
      sed -n '2,12p' "$0"; exit 0 ;;
    *)
      echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [[ -z "$PROJECT" ]]; then
  echo "error: --project is required" >&2
  exit 2
fi
if [[ $RESET -eq 0 && -z "$DELTA" ]]; then
  echo "error: provide --add <N> or --reset" >&2
  exit 2
fi
if [[ $RESET -eq 1 && -n "$DELTA" ]]; then
  echo "error: --reset cannot be combined with --add" >&2
  exit 2
fi
if [[ $RESET -eq 0 && ! "$DELTA" =~ ^-?[0-9]+$ ]]; then
  echo "error: --add value must be an integer" >&2
  exit 2
fi
if [[ $RESET -eq 0 && "$DELTA" == "0" ]]; then
  echo "error: --add 0 is a no-op" >&2
  exit 2
fi

API_URL="${LABKIT_API_URL:-http://localhost:8080}"
TOKEN="${LABKIT_ADMIN_TOKEN:-}"
if [[ -z "$TOKEN" ]]; then
  echo "error: LABKIT_ADMIN_TOKEN must be set" >&2
  exit 2
fi

if [[ $RESET -eq 1 ]]; then
  ACTION="reset bonus quota to 0 for ALL users in '$PROJECT'"
  ENDPOINT="$API_URL/api/admin/labs/$PROJECT/quota/bonus/reset"
  PREVIEW_PAYLOAD='{"dry_run": true}'
  APPLY_PAYLOAD=''
else
  if [[ "$DELTA" -gt 0 ]]; then
    ACTION="grant +$DELTA bonus quota to every user in '$PROJECT'"
  else
    ACTION="remove ${DELTA#-} bonus quota from every user in '$PROJECT' (floor at 0)"
  fi
  ENDPOINT="$API_URL/api/admin/labs/$PROJECT/quota/bonus"
  PREVIEW_PAYLOAD="{\"delta\": $DELTA, \"dry_run\": true}"
  APPLY_PAYLOAD="{\"delta\": $DELTA}"
fi

PREVIEW=$(curl -fsS -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "$PREVIEW_PAYLOAD" \
  "$ENDPOINT")
PARTICIPANTS=$(echo "$PREVIEW" | grep -oE '"lab_participants":[ ]*[0-9]+' | head -1 | grep -oE '[0-9]+' || echo "?")

echo "Plan: $ACTION."
echo "Target: $ENDPOINT"
echo "Lab participants (users with at least one submission): $PARTICIPANTS"
echo "Preview response: $PREVIEW"
if [[ "${ASSUME_YES:-0}" != "1" ]]; then
  read -r -p "Proceed? [y/N] " ANS
  if [[ "$ANS" != "y" && "$ANS" != "Y" ]]; then
    echo "aborted"
    exit 1
  fi
fi

if [[ -n "$APPLY_PAYLOAD" ]]; then
  RESPONSE=$(curl -fsS -X POST \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "$APPLY_PAYLOAD" \
    "$ENDPOINT")
else
  RESPONSE=$(curl -fsS -X POST \
    -H "Authorization: Bearer $TOKEN" \
    "$ENDPOINT")
fi

echo "$RESPONSE"
