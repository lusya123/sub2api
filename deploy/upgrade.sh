#!/bin/bash
# =============================================================================
# Sub2API Safe Upgrade Script
# =============================================================================
# This script safely upgrades the sub2api container without touching the database.
# It uses --no-deps to prevent postgres and redis from being recreated.
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
NC='\033[0m' # No Color

# Default image tag
IMAGE_TAG="${1:-custom}"

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

# Pull new image
echo -e "${YELLOW}Pulling new image: ghcr.io/lusya123/sub2api:${IMAGE_TAG}${NC}"
docker compose pull sub2api

# Upgrade sub2api only (--no-deps prevents postgres/redis recreation)
echo -e "${YELLOW}Upgrading sub2api container (database will not be touched)...${NC}"
docker compose up -d sub2api --no-deps

# Wait for health check
echo -e "${YELLOW}Waiting for service to be healthy...${NC}"
sleep 5

# Check if service is running
if docker ps --format '{{.Names}}' | grep -q '^sub2api$'; then
    echo -e "${GREEN}✓ Service is running${NC}"

    # Show new version
    echo ""
    echo -e "${YELLOW}New version:${NC}"
    docker exec sub2api /app/sub2api -version 2>/dev/null || echo "Unable to get version"

    # Show logs
    echo ""
    echo -e "${YELLOW}Recent logs:${NC}"
    docker compose logs --tail=20 sub2api

    echo ""
    echo -e "${GREEN}=== Upgrade completed successfully ===${NC}"
    echo -e "Service is running at: http://localhost:8080"
else
    echo -e "${RED}✗ Service failed to start${NC}"
    echo ""
    echo -e "${YELLOW}Logs:${NC}"
    docker compose logs --tail=50 sub2api
    exit 1
fi
