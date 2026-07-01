#!/bin/bash
# SweetPad always passes -workspace; bare .xcodeproj bundles need -project instead.
set -euo pipefail

xcodebuild="$(xcode-select -p)/usr/bin/xcodebuild"
args=()
while (($# > 0)); do
	if [[ "$1" == "-workspace" && "${2:-}" == *.xcodeproj ]]; then
		args+=(-project "$2")
		shift 2
	else
		args+=("$1")
		shift
	fi
done

exec "$xcodebuild" "${args[@]}"
