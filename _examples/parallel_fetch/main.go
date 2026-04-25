// Example: Parallel HTTP-like fetches using conquer.
//
// Demonstrates Scope.Go(), Async/Await, and error handling.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/cagatay35/conquer"
)

type User struct {
	ID   int
	Name string
}

type Order struct {
	ID     int
	UserID int
	Total  float64
}

func fetchUser(ctx context.Context, id int) (*User, error) {
	time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
	return &User{ID: id, Name: fmt.Sprintf("User-%d", id)}, nil
}

func fetchOrders(ctx context.Context, userID int) ([]Order, error) {
	time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
	return []Order{
		{ID: 1, UserID: userID, Total: 99.99},
		{ID: 2, UserID: userID, Total: 149.50},
	}, nil
}

func main() {
	ctx := context.Background()

	err := conquer.Run(ctx, func(s *conquer.Scope) error {
		userF := conquer.Async(s, func(ctx context.Context) (*User, error) {
			return fetchUser(ctx, 42)
		})
		ordersF := conquer.Async(s, func(ctx context.Context) ([]Order, error) {
			return fetchOrders(ctx, 42)
		})

		user, err := userF.Await(ctx)
		if err != nil {
			return fmt.Errorf("fetch user: %w", err)
		}

		orders, err := ordersF.Await(ctx)
		if err != nil {
			return fmt.Errorf("fetch orders: %w", err)
		}

		fmt.Printf("User: %s\n", user.Name)
		fmt.Printf("Orders: %d\n", len(orders))
		for _, o := range orders {
			fmt.Printf("  - Order #%d: $%.2f\n", o.ID, o.Total)
		}

		return nil
	}, conquer.WithTimeout(5*time.Second))

	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}
