ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/..)
SUPABASE_PROJECT := pisah

.PHONY: run test supabase-start supabase-stop supabase-logs

run:
	set -a && . ./.env && set +a && go run .

test:
	go test ./...

# analytics + vector stay enabled so Studio Logs Explorer works locally.
supabase-start:
	cd $(ROOT) && supabase start -x edge-runtime,functions,imgproxy,inbucket,meta,realtime,rest,storage --ignore-health-check

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
