package metrics

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	"google.golang.org/grpc"
)

func BenchmarkServerStream(b *testing.B) {
	statsCh := make(chan StatMessage, 1000)
	server := &IngestorServer{
		StatsCh: statsCh,
	}
	go server.ListenAndServe(":8082")
	//defer server.Shutdown()

	conn, err := grpc.Dial("localhost:8082", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		b.Fatal(err)
	}

	client := NewMetricIngestorClient(conn)
	sender, err := client.Ingest(context.Background(), grpc.FailFastCallOption{FailFast: true})
	if err != nil {
		b.Fatal(err)
	}

	statMsgs := makeMsgs(1)

	b.Run("encoding-sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, msg := range statMsgs {
				msg := WireStatMessage{
					Namespace: msg.Key.Namespace,
					Name:      msg.Key.Name,
					Stat:      &msg.Stat,
				}

				if err := sender.Send(&msg); err != nil {
					b.Fatal("Expected send to succeed, but got:", err)
				}
			}

			for j := 0; j < len(statMsgs); j++ {
				<-statsCh
			}
		}
	})
}

func BenchmarkServers(b *testing.B) {
	statsCh := make(chan StatMessage, 1000)
	server := &IngestorServer{
		StatsCh: statsCh,
	}
	go server.ListenAndServe(":8082")
	//defer server.Shutdown()

	conn, err := grpc.Dial("localhost:8082", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		b.Fatal(err)
	}
	client := NewMetricIngestorClient(conn)

	for _, numMsgs := range []int{1, 2, 10, 20, 50, 100} {
		statMsgs := makeMsgs(numMsgs)

		b.Run(fmt.Sprintf("grpc-unary-many-msgs-%d", numMsgs), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				msgs := make([]*WireStatMessage, 0, len(statMsgs))
				for _, statMsg := range statMsgs {
					msgs = append(msgs, &WireStatMessage{
						Namespace: statMsg.Key.Namespace,
						Name:      statMsg.Key.Name,
						Stat:      &statMsg.Stat,
					})
				}

				if _, err := client.IngestMany(context.Background(), &WireStatMessages{
					Messages: msgs,
				}); err != nil {
					b.Fatal("Expected send to succeed, but got:", err)
				}

				for j := 0; j < len(statMsgs); j++ {
					<-statsCh
				}
			}
		})

		b.Run(fmt.Sprintf("grpc-stream-single-msgs-%d", numMsgs), func(b *testing.B) {
			sender, err := client.Ingest(context.Background())
			if err != nil {
				b.Fatal(err)
			}

			for i := 0; i < b.N; i++ {
				for _, msg := range statMsgs {
					msg := WireStatMessage{
						Namespace: msg.Key.Namespace,
						Name:      msg.Key.Name,
						Stat:      &msg.Stat,
					}

					if err := sender.Send(&msg); err != nil {
						b.Fatal("Expected send to succeed, but got:", err)
					}
				}

				for j := 0; j < len(statMsgs); j++ {
					<-statsCh
				}
			}
		})

		b.Run(fmt.Sprintf("grpc-stream-many-msgs-%d", numMsgs), func(b *testing.B) {
			sender, err := client.IngestManyStream(context.Background())
			if err != nil {
				b.Fatal(err)
			}

			for i := 0; i < b.N; i++ {
				msgs := make([]*WireStatMessage, 0, len(statMsgs))
				for _, statMsg := range statMsgs {
					msgs = append(msgs, &WireStatMessage{
						Namespace: statMsg.Key.Namespace,
						Name:      statMsg.Key.Name,
						Stat:      &statMsg.Stat,
					})
				}

				if err := sender.Send(&WireStatMessages{
					Messages: msgs,
				}); err != nil {
					b.Fatal("Expected send to succeed, but got:", err)
				}

				for j := 0; j < len(statMsgs); j++ {
					<-statsCh
				}
			}
		})
	}
}

func makeMsgs(n int) []StatMessage {
	msgs := make([]StatMessage, 0, n)
	for i := 0; i < n; i++ {
		msgs = append(msgs, StatMessage{
			Key: types.NamespacedName{
				Namespace: "test-namespace",
				Name:      "test-revision",
			},
			Stat: Stat{
				PodName:                   "activator1",
				AverageConcurrentRequests: 2.1,
				RequestCount:              51,
			},
		})
	}
	return msgs
}
