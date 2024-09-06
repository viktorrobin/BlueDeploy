FROM golang:1.22.4-alpine AS build

WORKDIR /app
COPY . .

RUN go mod download
RUN go build -o /app/DeploymentManager

FROM alpine:latest

WORKDIR /app
COPY --from=build /app/DeploymentManager .

CMD ["./DeploymentManager"]