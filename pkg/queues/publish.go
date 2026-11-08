package queues

import (
	"log"
	"ttnmapper-services/pkg/utils"
)

func (context *PublishContext) Publish() {
	publisher, err := NewPublisher(context.QueueCredentials, context.Exchange)
	utils.FailOnError(err, "Connect publisher")
	defer publisher.Close()
	for message := range context.TxChannel {
		utils.FailOnError(publisher.PublishJSON(message, 0), "Publish message")
	}
	log.Fatal("Publish channel closed")
}
