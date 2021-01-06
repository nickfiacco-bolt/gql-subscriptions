package main

import (
	"context"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/graph-gophers/graphql-transport-ws/graphqlws"
	"github.com/rs/cors"
)

type resolver struct {
	events     chan string
	subscribers chan *subscriber
	unsubscribers chan *subscriber
}

type subscriber struct {
	id string
	events chan<- string
}

func (r *resolver) Hello() string {
    e := "hello"
    go func() {
        r.events <- e
    }()
    return e
}

func (_ *resolver) Bye() string {
	return "Bye"
}

func (r *resolver) HelloSaid(ctx context.Context) <-chan string {
    c := make(chan string)

	subscriber := &subscriber{id: uuid.New().String(), events: c}
	r.subscribers <- subscriber

	// add to channel when done to synchronize access to r.subscribers
	go func() {
		<- ctx.Done()
		r.unsubscribers <- subscriber
	}()

	return c
}

func newResolver() *resolver {
	r := &resolver{
	    events:     make(chan string),
		subscribers: make(chan *subscriber),
	}

	go r.handleSubscriptions()

	return r
}

func (r *resolver) handleSubscriptions() {
    subscribers := map[string]*subscriber{}

    for {
		select {
		case u := <-r.unsubscribers:
			delete(subscribers, u.id)
		case s := <-r.subscribers:
			subscribers[s.id] = s
		case e := <-r.events:
			for _, s := range subscribers {
				go func(s *subscriber) {
					s.events <- e
				}(s)
			}
		}
	}
}

func main() {
    s := `
        type Query {
            bye: String!
        }

        type Mutation {
            hello: String!
        }

        type Subscription {
		    helloSaid(): String!
	    }
    `
	mux := http.NewServeMux()
	schema := graphql.MustParseSchema(s, newResolver())
    graphQLHandler := graphqlws.NewHandlerFunc(schema, &relay.Handler{Schema: schema})
	mux.HandleFunc("/gql", graphQLHandler)
	handler := cors.AllowAll().Handler(mux)
	log.Fatal(http.ListenAndServe(":8080", handler))
}
