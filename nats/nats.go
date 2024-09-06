package nats

import (
	"context"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"log"
)

var (
	// NC is the global NATS connection
	NC *nats.Conn
)

// Connect initializes the global NATS connection
func Connect(natsURL string) {
	var err error
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}

	NC, err = nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v\n", err)
	}

	log.Println("NATS connection established:", natsURL)
}

// Close closes the global NATS connection
func Close() {
	if NC != nil {
		NC.Close()
		log.Println("NATS connection closed")
	}
}

func CreateDurableConsumer(ctx context.Context, jetstreamName string, consumerName string, filterSubject string) (jetstream.Consumer, error) {
	js, _ := jetstream.New(NC)
	stream, _ := js.Stream(ctx, jetstreamName)

	//log.Println("Connected to JetStream:", jetstreamName)
	//log.Println("Durable consumer name:", consumerName)
	//log.Println("Subjects filtered:", filterSubject)

	// Consumer - listen to the subject "Stack.*.*" and "Storage.*"
	cons, err := stream.CreateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: filterSubject,
	})

	return cons, err
}
