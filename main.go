package main

import (
	"DeploymentManager/deployment"
	"DeploymentManager/nats"
	"DeploymentManager/secrets"
	"DeploymentManager/utils"
	"context"
	"encoding/json"
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/nats-io/nats.go/jetstream"
	"log"
	"os"
	"time"
)

func initSecretManager() secrets.SecretManager {
	log.Println("Creating secret client...")
	start := time.Now()

	clientSecret, err := secrets.NewClientSecret(secrets.InfisicalConfig{
		ClientId:     os.Getenv("INFISICAL_CLIENT_ID"),
		ClientSecret: os.Getenv("INFISICAL_CLIENT_SECRET"),
		ProjectId:    os.Getenv("INFISICAL_PROJECT_ID"),
		Environment:  os.Getenv("INFISICAL_ENVIRONMENT"),
	})

	if err != nil {
		log.Fatalf("Error while creating secret client: %v\n", err)
	}

	err = clientSecret.LoadSecrets()

	if err != nil {
		log.Fatalf("Error loading secrets: %v\n", err)
	}

	log.Printf("Secret client created in %s", time.Since(start))

	return clientSecret
}

func initDockerClient(ctx context.Context) deployment.Docker {
	log.Println("Creating docker client")
	start := time.Now()

	dockerClient, err := deployment.NewClient(
		deployment.Configs{
			Host:     "unix:///var/run/deployment.sock",
			Registry: os.Getenv("DOCKER_PRIVATE_REGISTRY"),
			Username: os.Getenv("DOCKER_USERNAME"),
			Password: os.Getenv("DOCKER_PASSWORD"),
		})

	if err != nil {
		log.Fatalf("Error while creating deployment client: %v\n", err)
	}

	// Check login to private registry successful
	dockerClient.RegistryLogin(ctx)

	log.Printf("Docker client created in %s", time.Since(start))

	return dockerClient
}

func initNats(ctx context.Context) jetstream.Consumer {
	log.Println("Creating NATS JetStream consumer")
	start := time.Now()

	natsUrl := os.Getenv("NATS_URL")
	nats.Connect(natsUrl)

	// Create a JetStream context
	jetStreamName := os.Getenv("NATS_JETSTREAM_NAME")
	if jetStreamName == "" {
		log.Fatalf("NATS_JETSTREAM_NAME environment variable not set")
	}

	cons, err := nats.CreateDurableConsumer(ctx, jetStreamName, "DeploymentManager", "Stack.*.*")

	if err != nil {
		log.Fatalf("Error creating JetStream consumer: %v\n", err)
	}

	log.Printf("NATS JetStream consumer created in %s", time.Since(start))

	return cons

}

func main() {
	//slog.SetLogLoggerLevel(slog.LevelDebug)

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	defer nats.Close()

	ctx := context.Background()

	// Initialise Secret Manager
	secretChan := make(chan secrets.SecretManager)
	go func() {
		clientSecret := initSecretManager()
		secretChan <- clientSecret
	}()

	// Initialise docker client
	dockerChan := make(chan deployment.Docker)
	go func() {
		dockerClient := initDockerClient(ctx)
		dockerChan <- dockerClient
	}()

	// Initialise NATS
	dockerNats := make(chan jetstream.Consumer)
	go func() {
		consNats := initNats(ctx)
		dockerNats <- consNats
	}()

	clientSecret := <-secretChan
	dockerClient := <-dockerChan
	consumer := <-dockerNats

	// Create the consumer to listen to the JetStream
	consumerInfo, err := consumer.Info(ctx)

	log.Println("Connected to JetStream:", consumerInfo.Stream)
	log.Println("Durable consumer name:", consumerInfo.Name)
	log.Println("Subjects filtered:", consumerInfo.Config.FilterSubject)
	log.Println("Messages pending:", consumerInfo.NumPending)
	log.Println("Messages pending acknowledgement:", consumerInfo.NumAckPending)

	iter, err := consumer.Messages()
	if err != nil {
		log.Fatalf("Error creating JetStream iterator: %v\n", err)
	}

	log.Println("Ready to listen...")

	// Start the event loop
	for {
		msg, err := iter.Next()

		if err != nil {
			log.Printf("Error reading JetStream message: %v\n", err)
			break
		}

		var event = cloudevents.NewEvent()
		err = json.Unmarshal([]byte(msg.Data()), &event)

		if err != nil {
			log.Printf("Error unmarshalling event: %v\n", err)
			break
		}

		log.Println("Event Subject: ", event.Type())
		log.Println("Event ID: ", event.ID())
		log.Println("Event Source: ", event.Source())

		switch {

		case msg.Subject() == "Stack.Containers.ImageCreated":

			// Process the new image created
			go processNewImageCreated(ctx, dockerClient, event)

		case msg.Subject() == "Stack.Secrets.NewSecret2":
			// Reload the secrets
			clientSecret.LoadSecrets()

			// Loop over running containers
			err = dockerClient.RecreateRunningContainers(ctx)
			if err != nil {
				log.Printf("Error recreating running containers: %v\n", err)
			}

		default:
			log.Printf("Received a JetStream message: %s\n", string(msg.Data()))
		}

		err = msg.Ack()

		if err != nil {
			log.Printf("Error acknowledging message: %v\n", err)
		}
	}

}

func processNewImageCreated(ctx context.Context, dockerClient deployment.Docker, event cloudevents.Event) {
	request := deployment.DeploymentRequest{}
	err := json.Unmarshal(event.Data(), &request)

	if err != nil {
		log.Printf("Error parsing the event data: %v\n", err)
	}

	log.Printf("Received a request to deploy container image: %v\n", request.Container.Image)

	containerId, err := dockerClient.DeployContainer(ctx, request)
	if err != nil {
		log.Printf("Error deploying container: %v\n", err)
	}

	//Save the request object to directory /deployments
	if containerId == "" {
		log.Printf("Error deploying container: %v\n", err)
	} else {
		fileName := containerId + ".gob"
		err = utils.SaveToFile(fileName, request)
		if err != nil {
			fmt.Println("Error saving object:", err)
		}
	}
}
