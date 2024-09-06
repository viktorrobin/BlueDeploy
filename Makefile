IMAGE_NAME=docker.bluerobin.io/deployment-manager:latest

login:
	docker login docker.bluerobin.io -u ${DOCKER_BLUEROBIN_USERNAME} -p ${DOCKER_BLUEROBIN_PASSWORD}

build:
	docker build --platform linux/amd64 -t ${IMAGE_NAME} . 

push:
	docker push ${IMAGE_NAME}
	
run: build
	docker run --network bluerobin --hostname stack-manager --name stack-manager1 --rm -e NATS_URL=${NATS_URL} ${IMAGE_NAME}