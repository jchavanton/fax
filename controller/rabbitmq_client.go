package main

import (
	"context"
	"fmt"
	"strings"
	"os"
	"time"
	amqp "github.com/rabbitmq/amqp091-go"
)

// func rmqPublish(queue string, report string, key string) {
func rmqPublish(report string, key string) {
	rmqIp := os.Getenv("RMQ_IP")
	rmqUsername := os.Getenv("RMQ_USERNAME")
	rmqPassword := os.Getenv("RMQ_PASSWORD")
	rmq := "amqp://"+rmqUsername+":"+rmqPassword+"@"+rmqIp+":5672/"
	fmt.Printf("rmqPublish [%s] exchange[%s] key[%s]", rmq, os.Getenv("RMQ_PUB_EXCHANGE"), key)
	conn, err := amqp.Dial(rmq)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}
	defer ch.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = ch.PublishWithContext(ctx,
		os.Getenv("RMQ_PUB_EXCHANGE"), // exchange
		key,    // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing {
			ContentType: "text/plain",
			Body:        []byte(report),
		})
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}
	fmt.Printf(" [x] Sent %s\n", report)
}

func rmqSubscribe(cmdQ *[]Cmd, q string) {
	rmqIp := os.Getenv("RMQ_IP")
	rmqUsername := os.Getenv("RMQ_USERNAME")
	rmqPassword := os.Getenv("RMQ_PASSWORD")
	rmq := "amqp://"+rmqUsername+":"+rmqPassword+"@"+rmqIp+":5672/"
	fmt.Printf("rmq consumer [%s]queue[%s]\n", rmq, q)
	conn, err := amqp.Dial(rmq)

	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}
	defer ch.Close()

	msgs, err := ch.Consume(
		q, // name
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		fmt.Printf("error [%s]\n", err.Error())
	}

	// context of the test origin
	context := ""
	if strings.Contains("customer",strings.ToLower(q)) {
		context = "customer"
	} else if strings.Contains("provider",strings.ToLower(q)) {
		context = "provider"
	} else {
		context = ""
	}

	var forever chan struct{}
	go func() {
		for d := range msgs {
			fmt.Printf("command message received: %s\n", d.Body)
			_, err := cmdCreate(string(d.Body[:]), cmdQ, context)
			if err != nil {
				fmt.Printf("cmdCreate: message received error:", err)
			}
		}
	} ()

	fmt.Printf(" [*] Waiting for messages on queue [%s]\n", q)
	<-forever
}
