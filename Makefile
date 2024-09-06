IMAGE_NAME=${DOCKER_REGISTRY}/blue-deploy:latest

login:
	docker login ${DOCKER_REGISTRY} -u ${DOCKER_USERNAME} -p ${DOCKER_PASSWORD}

build:
	docker build --platform linux/amd64 -t ${IMAGE_NAME} . 

push:
	docker push ${IMAGE_NAME}
	
run: build
	docker run --network bluerobin --hostname stack-manager --name stack-manager1 --rm -e NATS_URL=${NATS_URL} ${IMAGE_NAME}