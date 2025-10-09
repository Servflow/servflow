HOST := servflow
IMAGE_NAME := servflow
TAG := $(shell git describe --tags --abbrev=0)
DOCKER_IMAGE := $(HOST)/$(IMAGE_NAME):$(TAG)

docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(DOCKER_IMAGE) \
		--build-arg GIT_USERNAME=$(GIT_USERNAME) \
		--build-arg GIT_PASSWORD=$(GIT_PASSWORD) .

docker-push:
	docker push $(DOCKER_IMAGE)
