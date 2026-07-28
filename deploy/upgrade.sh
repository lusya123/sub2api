#!/bin/bash
# =============================================================================
# Sub2API Safe Upgrade Script
# =============================================================================
# This script safely upgrades the sub2api container without touching the database.
# It uses --no-deps to prevent postgres and redis from being recreated.
#
# IMPORTANT: This script automatically backs up all data before upgrading.
# Each backup is timestamped and stored in ~/recovery/
#
# Usage:
#   ./upgrade.sh [image_tag]
#
# Examples:
#   ./upgrade.sh                    # Pull and upgrade to :custom tag
#   ./upgrade.sh custom-abc1234     # Upgrade to specific tag
# =============================================================================

set -euo pipefail
umask 077

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default image tag
IMAGE_TAG="${1:-custom}"
SUB2API_IMAGE="ghcr.io/lusya123/sub2api:${IMAGE_TAG}"
export SUB2API_IMAGE

# Pin every Compose command to the configuration that owns the running
# container. This prevents a same-named project in another directory from
# being upgraded accidentally.
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"
ENV_FILE="${SCRIPT_DIR}/.env"

# Backup directory with timestamp
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_DIR="$HOME/recovery/sub2api-backup-$TIMESTAMP"

echo -e "${GREEN}=== Sub2API Safe Upgrade ===${NC}"
echo ""

# Check that the script's Compose configuration exists
if [ ! -f "$COMPOSE_FILE" ]; then
    echo -e "${RED}Error: docker-compose.yml not found${NC}"
    echo "Expected: $COMPOSE_FILE"
    exit 1
fi

# Check that the expected deployment containers exist
for container_name in sub2api sub2api-postgres sub2api-redis; do
    if ! docker ps -a --format '{{.Names}}' | grep -q "^${container_name}$"; then
        echo -e "${RED}Error: ${container_name} container not found${NC}"
        echo "Please ensure the service is deployed first"
        exit 1
    fi
done

COMPOSE_PROJECT="$(docker inspect --format '{{ index .Config.Labels "com.docker.compose.project" }}' sub2api)"
COMPOSE_WORKING_DIR="$(docker inspect --format '{{ index .Config.Labels "com.docker.compose.project.working_dir" }}' sub2api)"
COMPOSE_CONFIG_FILES="$(docker inspect --format '{{ index .Config.Labels "com.docker.compose.project.config_files" }}' sub2api)"
if [ -z "$COMPOSE_PROJECT" ] || [ -z "$COMPOSE_WORKING_DIR" ] || [ -z "$COMPOSE_CONFIG_FILES" ]; then
    echo -e "${RED}Error: running sub2api container is missing Compose ownership labels${NC}"
    exit 1
fi

if ! NORMALIZED_COMPOSE_WORKING_DIR="$(cd -- "$COMPOSE_WORKING_DIR" 2>/dev/null && pwd -P)"; then
    echo -e "${RED}Error: deployed Compose working directory does not exist: ${COMPOSE_WORKING_DIR}${NC}"
    exit 1
fi
if [ "$NORMALIZED_COMPOSE_WORKING_DIR" != "$SCRIPT_DIR" ]; then
    echo -e "${RED}Error: this script does not own the running sub2api container${NC}"
    echo "Container working directory: $NORMALIZED_COMPOSE_WORKING_DIR"
    echo "Script working directory:    $SCRIPT_DIR"
    exit 1
fi

IFS=',' read -r -a DEPLOYED_COMPOSE_FILES <<< "$COMPOSE_CONFIG_FILES"
if [ "${#DEPLOYED_COMPOSE_FILES[@]}" -ne 1 ]; then
    echo -e "${RED}Error: deployment uses multiple Compose files; refusing to recreate it with an incomplete model${NC}"
    printf 'Deployed file: %s\n' "${DEPLOYED_COMPOSE_FILES[@]}"
    exit 1
fi
deployed_compose_file="${DEPLOYED_COMPOSE_FILES[0]}"
if ! NORMALIZED_DEPLOYED_COMPOSE_FILE="$(
    cd -- "$(dirname -- "$deployed_compose_file")" 2>/dev/null &&
    printf '%s/%s\n' "$(pwd -P)" "$(basename -- "$deployed_compose_file")"
)"; then
    echo -e "${RED}Error: deployed Compose file cannot be resolved: ${deployed_compose_file}${NC}"
    exit 1
fi
if [ "$NORMALIZED_DEPLOYED_COMPOSE_FILE" != "$COMPOSE_FILE" ]; then
    echo -e "${RED}Error: running container was created from a different Compose file${NC}"
    echo "Container config file: $NORMALIZED_DEPLOYED_COMPOSE_FILE"
    echo "Script config file:    $COMPOSE_FILE"
    exit 1
fi

for ownership in sub2api:sub2api sub2api-postgres:postgres sub2api-redis:redis; do
    container_name="${ownership%%:*}"
    expected_service="${ownership#*:}"
    container_project="$(docker inspect --format '{{ index .Config.Labels "com.docker.compose.project" }}' "$container_name")"
    container_working_dir="$(docker inspect --format '{{ index .Config.Labels "com.docker.compose.project.working_dir" }}' "$container_name")"
    container_config_files="$(docker inspect --format '{{ index .Config.Labels "com.docker.compose.project.config_files" }}' "$container_name")"
    container_service="$(docker inspect --format '{{ index .Config.Labels "com.docker.compose.service" }}' "$container_name")"
    if [ "$container_project" != "$COMPOSE_PROJECT" ]; then
        echo -e "${RED}Error: ${container_name} belongs to Compose project '${container_project}', expected '${COMPOSE_PROJECT}'${NC}"
        exit 1
    fi
    if [ "$container_working_dir" != "$COMPOSE_WORKING_DIR" ] || [ "$container_config_files" != "$COMPOSE_CONFIG_FILES" ]; then
        echo -e "${RED}Error: ${container_name} does not share the application's Compose configuration${NC}"
        exit 1
    fi
    if [ "$container_service" != "$expected_service" ]; then
        echo -e "${RED}Error: ${container_name} has Compose service '${container_service}', expected '${expected_service}'${NC}"
        exit 1
    fi
done

COMPOSE=(
    docker compose
    --project-name "$COMPOSE_PROJECT"
    --project-directory "$SCRIPT_DIR"
    --file "$COMPOSE_FILE"
)
POSTGRES_CONTAINER_ID_BEFORE="$(docker inspect --format '{{.Id}}' sub2api-postgres)"
REDIS_CONTAINER_ID_BEFORE="$(docker inspect --format '{{.Id}}' sub2api-redis)"

# Show current version
echo -e "${YELLOW}Current version:${NC}"
docker exec sub2api /app/sub2api -version 2>/dev/null || echo "Unable to get version"
echo ""

# =============================================================================
# STEP 1: Backup all data before upgrade
# =============================================================================
echo -e "${CYAN}=== Step 1: Backing up all data ===${NC}"
echo -e "${YELLOW}Backup directory: ${BACKUP_DIR}${NC}"
echo ""

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Backup configuration files
echo -e "${YELLOW}[1/4] Backing up configuration files...${NC}"
cp "$COMPOSE_FILE" "$BACKUP_DIR/"
if [ -f "$ENV_FILE" ]; then
    cp "$ENV_FILE" "$BACKUP_DIR/"
else
    echo "Warning: .env not found"
fi

# Backup PostgreSQL database
echo -e "${YELLOW}[2/4] Backing up PostgreSQL database...${NC}"
POSTGRES_USER="$(docker exec sub2api-postgres sh -c 'printf %s "${POSTGRES_USER:-sub2api}"')"
POSTGRES_DB="$(docker exec sub2api-postgres sh -c 'printf %s "${POSTGRES_DB:-sub2api}"')"
docker exec sub2api-postgres pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" > "$BACKUP_DIR/sub2api_db.sql"
DB_SIZE=$(du -h "$BACKUP_DIR/sub2api_db.sql" | cut -f1)
echo -e "${GREEN}✓ Database backup completed (${DB_SIZE})${NC}"

# Backup Redis data
echo -e "${YELLOW}[3/4] Backing up Redis data...${NC}"
docker exec sub2api-redis redis-cli save 2>/dev/null || true
docker cp sub2api-redis:/data/dump.rdb "$BACKUP_DIR/redis_dump.rdb" 2>/dev/null || echo "Warning: Redis backup skipped"

# Backup application data volume
echo -e "${YELLOW}[4/4] Backing up application data volume...${NC}"
APP_DATA_VOLUME="$(docker inspect --format '{{ range .Mounts }}{{ if eq .Destination "/app/data" }}{{ .Name }}{{ end }}{{ end }}' sub2api)"
if [ -z "$APP_DATA_VOLUME" ] || ! docker volume inspect "$APP_DATA_VOLUME" >/dev/null 2>&1; then
    echo -e "${RED}Error: unable to resolve the sub2api /app/data volume${NC}"
    exit 1
fi
docker run --rm -v "${APP_DATA_VOLUME}:/data:ro" -v "$BACKUP_DIR":/backup alpine \
    tar czf /backup/sub2api_data.tar.gz -C /data .

echo ""
echo -e "${GREEN}✓ Backup completed successfully${NC}"
echo -e "${CYAN}Backup location: ${BACKUP_DIR}${NC}"
BACKUP_TOTAL_SIZE=$(du -sh "$BACKUP_DIR" | cut -f1)
echo -e "${CYAN}Total backup size: ${BACKUP_TOTAL_SIZE}${NC}"
echo ""

# =============================================================================
# STEP 2: Upgrade
# =============================================================================
echo -e "${CYAN}=== Step 2: Upgrading sub2api ===${NC}"
echo ""

# Pull new image
echo -e "${YELLOW}Pulling new image: ${SUB2API_IMAGE}${NC}"
"${COMPOSE[@]}" pull sub2api
EXPECTED_IMAGE_ID="$(docker image inspect "$SUB2API_IMAGE" --format '{{.Id}}')"

# Upgrade sub2api only (--no-deps prevents postgres/redis recreation)
echo -e "${YELLOW}Upgrading sub2api container (database will not be touched)...${NC}"
"${COMPOSE[@]}" up -d sub2api --no-deps

# --no-deps must leave the stateful containers byte-for-byte identical.
POSTGRES_CONTAINER_ID_AFTER="$(docker inspect --format '{{.Id}}' sub2api-postgres)"
REDIS_CONTAINER_ID_AFTER="$(docker inspect --format '{{.Id}}' sub2api-redis)"
if [ "$POSTGRES_CONTAINER_ID_AFTER" != "$POSTGRES_CONTAINER_ID_BEFORE" ] ||
    [ "$REDIS_CONTAINER_ID_AFTER" != "$REDIS_CONTAINER_ID_BEFORE" ]; then
    echo -e "${RED}Error: a stateful dependency container changed during application-only upgrade${NC}"
    exit 1
fi

# Verify Compose started the exact image that was pulled
RUNNING_IMAGE_ID="$(docker inspect --format '{{.Image}}' sub2api)"
if [ "$RUNNING_IMAGE_ID" != "$EXPECTED_IMAGE_ID" ]; then
    echo -e "${RED}Error: running container image does not match the pulled image${NC}"
    echo "Expected image ID: $EXPECTED_IMAGE_ID"
    echo "Running image ID:  $RUNNING_IMAGE_ID"
    "${COMPOSE[@]}" logs --tail=50 sub2api
    exit 1
fi

# Wait for the configured health check rather than treating "running" as ready.
echo -e "${YELLOW}Waiting for service to be healthy...${NC}"
HEALTH_TIMEOUT_SECONDS="${HEALTH_TIMEOUT_SECONDS:-240}"
if ! [[ "$HEALTH_TIMEOUT_SECONDS" =~ ^[1-9][0-9]*$ ]]; then
    echo -e "${RED}Error: HEALTH_TIMEOUT_SECONDS must be a positive integer${NC}"
    exit 1
fi

deadline=$((SECONDS + HEALTH_TIMEOUT_SECONDS))
HEALTH_STATE=""
RUN_STATE=""
while [ "$SECONDS" -lt "$deadline" ]; do
    RUN_STATE="$(docker inspect --format '{{.State.Status}}' sub2api)"
    HEALTH_STATE="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' sub2api)"
    if [ "$RUN_STATE" != "running" ]; then
        break
    fi
    if [ "$HEALTH_STATE" = "healthy" ]; then
        break
    fi
    if [ "$HEALTH_STATE" = "unhealthy" ]; then
        break
    fi
    sleep 5
done

if [ "$RUN_STATE" != "running" ] || [ "$HEALTH_STATE" != "healthy" ]; then
    echo -e "${RED}✗ Service failed its health check (container=${RUN_STATE}, health=${HEALTH_STATE})${NC}"
    echo ""
    echo -e "${YELLOW}Logs:${NC}"
    "${COMPOSE[@]}" logs --tail=50 sub2api
    echo ""
    echo -e "${YELLOW}You can restore from backup at: ${BACKUP_DIR}${NC}"
    exit 1
fi

# =============================================================================
# STEP 3: Verify upgrade
# =============================================================================
echo ""
echo -e "${CYAN}=== Step 3: Verifying upgrade ===${NC}"
echo ""

echo -e "${GREEN}✓ Service is healthy${NC}"

# Show new version
echo ""
echo -e "${YELLOW}New version:${NC}"
docker exec sub2api /app/sub2api -version 2>/dev/null || echo "Unable to get version"

# Verify database integrity
echo ""
echo -e "${YELLOW}Verifying database integrity...${NC}"
USER_COUNT=$(docker exec sub2api-postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -c 'SELECT COUNT(*) FROM users;' 2>/dev/null | tr -d ' ')
ACCOUNT_COUNT=$(docker exec sub2api-postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -c 'SELECT COUNT(*) FROM accounts;' 2>/dev/null | tr -d ' ')
echo -e "${GREEN}✓ Database intact: ${USER_COUNT} users, ${ACCOUNT_COUNT} accounts${NC}"

# Show logs
echo ""
echo -e "${YELLOW}Recent logs:${NC}"
"${COMPOSE[@]}" logs --tail=20 sub2api

echo ""
echo -e "${GREEN}=== Upgrade completed successfully ===${NC}"
echo -e "Service is running at: http://localhost:8080"
echo ""
echo -e "${CYAN}Backup saved at: ${BACKUP_DIR}${NC}"
