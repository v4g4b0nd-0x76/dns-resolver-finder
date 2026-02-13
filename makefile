IMAGE_NAME := dns-resolver-finder
UBUNTU_BIN_DIR := bins/ubuntu_22_04

.PHONY: all docker-build ubuntu-build clean

all: docker-build ubuntu-build

# Build and run as a standard Docker container
docker-build:
	docker build -f dockerfiles/Dockerfile -t  $(IMAGE_NAME):latest .
	@echo "Docker image $(IMAGE_NAME):latest built."

# Build specifically for Ubuntu 22.04 and extract binary
ubuntu-build:
	@mkdir -p $(UBUNTU_BIN_DIR)
	@echo "Building for Ubuntu 22.04 environment..."
	docker build -f dockerfiles/Dockerfile.ubuntu -t $(IMAGE_NAME)-ubuntu-builder .
	# Create a temporary container to extract the binary
	$(eval CONTAINER_ID=$(shell docker create $(IMAGE_NAME)-ubuntu-builder))
	docker cp $(CONTAINER_ID):/build/dns-scanner $(UBUNTU_BIN_DIR)/dns-scanner
	docker rm $(CONTAINER_ID)
	@echo "Binary copied to $(UBUNTU_BIN_DIR)/dns-scanner"

clean:
	rm -rf bins/
	docker rmi $(IMAGE_NAME):latest $(IMAGE_NAME)-ubuntu-builder || true
