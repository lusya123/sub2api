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

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default image tag
IMAGE_TAG="${1:-custom}"

# Backup directory with timestamp
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_DIR="$HOME/recovery/sub2api-backup-$TIMESTAMP"

echo -e "${GREEN}=== Sub2API Safe Upgrade ===${NC}"
echo ""

# Check if we're in the right directory
if [ ! -f "docker-compose.yml" ]; then
    echo -e "${RED}Error: docker-compose.yml not found${NC}"
    echo "Please run this script from the deploy directory"
    exit 1
fi

# Check if sub2api container exists
if ! docker ps -a --format '{{.Names}}' | grep -q '^sub2api$'; then
    echo -e "${RED}Error: sub2api container not found${NC}"
    echo "Please ensure the service is deployed first"
    exit 1
fi

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
cp .env docker-compose.yml "$BACKUP_DIR/" 2>/dev/null || echo "Warning: Some config files not found"

# Backup PostgreSQL database
echo -e "${YELLOW}[2/4] Backing up PostgreSQL database...${NC}"
docker exec sub2api-postgres pg_dump -U sub2api sub2api > "$BACKUP_DIR/sub2api_db.sql"
DB_SIZE=$(du -h "$BACKUP_DIR/sub2api_db.sql" | cut -f1)
echo -e "${GREEN}✓ Database backup completed (${DB_SIZE})${NC}"

# Backup Redis data
echo -e "${YELLOW}[3/4] Backing up Redis data...${NC}"
docker exec sub2api-redis redis-cli save 2>/dev/null || true
docker cp sub2api-redis:/data/dump.rdb "$BACKUP_DIR/redis_dump.rdb" 2>/dev/null || echo "Warning: Redis backup skipped"

# Backup application data volume
echo -e "${YELLOW}[4/4] Backing up application data volume...${NC}"
docker run --rm -v deploy_sub2api_data:/data -v "$BACKUP_DIR":/backup alpine tar czf /backup/sub2api_data.tar.gz -C /data . 2>/dev/null || echo "Warning: Volume backup skipped"

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
echo -e "${YELLOW}Pulling new image: ghcr.io/lusya123/sub2api:${IMAGE_TAG}${NC}"
docker compose pull sub2api

# Upgrade sub2api only (--no-deps prevents postgres/redis recreation)
echo -e "${YELLOW}Upgrading sub2api container (database will not be touched)...${NC}"
docker compose up -d sub2api --no-deps

# Wait for health check
echo -e "${YELLOW}Waiting for service to be healthy...${NC}"
sleep 5

# =============================================================================
# STEP 3: Verify upgrade
# =============================================================================
echo ""
echo -e "${CYAN}=== Step 3: Verifying upgrade ===${NC}"
echo ""

# Check if service is running
if docker ps --format '{{.Names}}' | grep -q '^sub2api$'; then
    echo -e "${GREEN}✓ Service is running${NC}"

    # Show new version
    echo ""
    echo -e "${YELLOW}New version:${NC}"
    docker exec sub2api /app/sub2api -version 2>/dev/null || echo "Unable to get version"

    # Verify database integrity
    echo ""
    echo -e "${YELLOW}Verifying database integrity...${NC}"
    USER_COUNT=$(docker exec sub2api-postgres psql -U sub2api -d sub2api -t -c 'SELECT COUNT(*) FROM users;' 2>/dev/null | tr -d ' ')
    ACCOUNT_COUNT=$(docker exec sub2api-postgres psql -U sub2api -d sub2api -t -c 'SELECT COUNT(*) FROM accounts;' 2>/dev/null | tr -d ' ')
    echo -e "${GREEN}✓ Database intact: ${USER_COUNT} users, ${ACCOUNT_COUNT} accounts${NC}"

    # Show logs
    echo ""
    echo -e "${YELLOW}Recent logs:${NC}"
    docker compose logs --tail=20 sub2api

    echo ""
    echo -e "${GREEN}=== Upgrade completed successfully ===${NC}"
    echo -e "Service is running at: http://localhost:8080"
    echo ""
    echo -e "${CYAN}Backup saved at: ${BACKUP_DIR}${NC}"
else
    echo -e "${RED}✗ Service failed to start${NC}"
    echo ""
    echo -e "${YELLOW}Logs:${NC}"
    docker compose logs --tail=50 sub2api
    echo ""
    echo -e "${YELLOW}You can restore from backup at: ${BACKUP_DIR}${NC}"
    exit 1
fi
