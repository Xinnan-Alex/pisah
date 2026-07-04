ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
SUPABASE_PROJECT := pisah

.PHONY: run local mobile test supabase-start supabase-stop supabase-logs

# Default: localhost only. Use `make mobile` (or `make run mobile`) for phone access on LAN.
run: local

local:
	@set -a && . ./.env && set +a && \
	port=$${PORT:-8080} && \
	echo "Starting on http://127.0.0.1:$$port" && \
	HOST=127.0.0.1 PUBLIC_BASE_URL=http://127.0.0.1:$$port go run .

mobile:
	@set -a && . ./.env && set +a && \
	ip=$$(ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null) && \
	port=$${PORT:-8080} && \
	if [ -z "$$ip" ]; then \
		echo "Could not detect LAN IP. Try: HOST=0.0.0.0 PUBLIC_BASE_URL=http://YOUR_IP:$$port go run ."; \
		exit 1; \
	fi && \
	echo "Starting on http://$$ip:$$port (open this on your phone, same Wi-Fi)" && \
	HOST=0.0.0.0 PUBLIC_BASE_URL=http://$$ip:$$port go run .

test:
	go test ./...

# analytics + vector stay enabled so Studio Logs Explorer works locally.
supabase-start:
	cd $(ROOT) && supabase start -x edge-runtime,functions,imgproxy,inbucket,meta,realtime,rest --ignore-health-check

supabase-stop:
	cd $(ROOT) && supabase stop

supabase-logs:
	@containers=$$(docker ps --filter label=com.supabase.cli.project=$(SUPABASE_PROJECT) --format '{{.Names}}' | sort); \
	if [ -z "$$containers" ]; then \
		echo "No Supabase containers running for project $(SUPABASE_PROJECT). Run 'make supabase-start' first."; \
		exit 1; \
	fi; \
	echo "Tailing: $$containers (Ctrl+C to stop)"; \
	trap 'kill 0' INT TERM; \
	for c in $$containers; do \
		docker logs -f --tail=100 "$$c" 2>&1 | awk -v c="$$c" '{print "[" c "] " $$0}' & \
	done; \
	wait
