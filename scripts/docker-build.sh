#!/bin/bash

# Docker Build Script for Updater Service
# Builds secure Docker images with proper tagging and security scanning

set -e

# Configuration
REGISTRY="${DOCKER_REGISTRY:-localhost}"
IMAGE_NAME="${IMAGE_NAME:-updater}"
VERSION="${VERSION:-$(git rev-parse --short HEAD)}"
BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🐳 Docker Build Script for Updater Service${NC}"
echo "============================================="
echo -e "Registry: ${YELLOW}${REGISTRY}${NC}"
echo -e "Image: ${YELLOW}${IMAGE_NAME}${NC}"
echo -e "Version: ${YELLOW}${VERSION}${NC}"
echo ""

# Function to build image
build_image() {
    local dockerfile=$1
    local tag_suffix=$2
    local description=$3

    echo -e "${BLUE}Building ${description}...${NC}"

    docker build \
        -f "${dockerfile}" \
        --build-arg BUILD_DATE="${BUILD_DATE}" \
        --build-arg VERSION="${VERSION}" \
        --build-arg VCS_REF="$(git rev-parse HEAD)" \
        -t "${REGISTRY}/${IMAGE_NAME}:${VERSION}${tag_suffix}" \
        -t "${REGISTRY}/${IMAGE_NAME}:latest${tag_suffix}" \
        .

    echo -e "${GREEN}✅ Built ${description} successfully${NC}"
    echo ""
}

# Function to scan image for vulnerabilities
scan_image() {
    local image_tag=$1

    echo -e "${BLUE}🔍 Scanning ${image_tag} for vulnerabilities...${NC}"

    if command -v trivy &> /dev/null; then
        trivy image --severity HIGH,CRITICAL "${image_tag}"
        echo ""
    else
        echo -e "${YELLOW}⚠️  Trivy not found, skipping vulnerability scan${NC}"
        echo "Install trivy: https://aquasecurity.github.io/trivy/latest/getting-started/installation/"
        echo ""
    fi
}

# Function to inspect image
inspect_image() {
    local image_tag=$1

    echo -e "${BLUE}🔍 Inspecting ${image_tag}...${NC}"

    # Get image size
    local size=$(docker image inspect "${image_tag}" --format='{{.Size}}' | numfmt --to=iec)

    # Get layer count
    local layers=$(docker history "${image_tag}" --format="{{.ID}}" | wc -l)

    # Check if image runs as non-root
    local user=$(docker image inspect "${image_tag}" --format='{{.Config.User}}')

    echo -e "Size: ${YELLOW}${size}${NC}"
    echo -e "Layers: ${YELLOW}${layers}${NC}"
    echo -e "User: ${YELLOW}${user:-root}${NC}"

    if [[ "${user}" != "root" && "${user}" != "" ]]; then
        echo -e "Security: ${GREEN}✅ Non-root user${NC}"
    else
        echo -e "Security: ${RED}❌ Running as root${NC}"
    fi

    echo ""
}

# Parse command line arguments
DOCKERFILE="Dockerfile"
BUILD_TYPE="standard"

while [[ $# -gt 0 ]]; do
    case $1 in
        --scan)
            ENABLE_SCAN=true
            shift
            ;;
        --push)
            PUSH_IMAGE=true
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --scan      Run vulnerability scan after build"
            echo "  --push      Push image to registry after build"
            echo "  --help      Show this help message"
            echo ""
            echo "Environment Variables:"
            echo "  DOCKER_REGISTRY   Docker registry (default: localhost)"
            echo "  IMAGE_NAME        Image name (default: updater)"
            echo "  VERSION           Image version (default: git short hash)"
            echo ""
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Check if Docker is running
if ! docker info &> /dev/null; then
    echo -e "${RED}❌ Docker is not running${NC}"
    exit 1
fi

# Build the image
build_image "${DOCKERFILE}" "" "secure image (distroless-based)"
IMAGE_TAG="${REGISTRY}/${IMAGE_NAME}:${VERSION}"

# Inspect the built image
inspect_image "${IMAGE_TAG}"

# Run vulnerability scan if requested
if [[ "${ENABLE_SCAN}" == "true" ]]; then
    scan_image "${IMAGE_TAG}"
fi

# Push image if requested
if [[ "${PUSH_IMAGE}" == "true" ]]; then
    echo -e "${BLUE}📤 Pushing ${IMAGE_TAG} to registry...${NC}"
    docker push "${IMAGE_TAG}"
    docker push "${REGISTRY}/${IMAGE_NAME}:latest"
    echo -e "${GREEN}✅ Image pushed successfully${NC}"
fi

echo -e "${GREEN}🎉 Build process completed successfully!${NC}"
echo ""
echo "Built images:"
echo "  • ${REGISTRY}/${IMAGE_NAME}:${VERSION}"
echo "  • ${REGISTRY}/${IMAGE_NAME}:latest"
echo ""
echo "Run the image:"
echo "  # Development"
echo "  docker run --read-only --cap-drop=ALL --security-opt=no-new-privileges:true \\"
echo "    -p 8080:8080 -e UPDATER_CONFIG_SECTION=development ${REGISTRY}/${IMAGE_NAME}:${VERSION}"
echo ""
echo "  # Production"
echo "  docker run --read-only --cap-drop=ALL --security-opt=no-new-privileges:true \\"
echo "    -p 8080:8080 -e UPDATER_CONFIG_SECTION=production --env-file=.env.prod ${REGISTRY}/${IMAGE_NAME}:${VERSION}"